package ouroboros

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/agentport"
)

// HintFeedback is the artifact emitted by the scaffolded judge when hints
// remain and the subject's answer is insufficient.
type HintFeedback struct {
	HintIndex      int    `json:"hint_index"`
	Hint           string `json:"hint"`
	OriginalPrompt string `json:"original_prompt"`
	PriorResponse  string `json:"prior_response"`
}

// ScaffoldedNodes returns a NodeRegistry for the scaffolded prompt-chain circuit.
func ScaffoldedNodes(seed *Seed, dispatch ProbeDispatcher) engine.NodeRegistry {
	return ScaffoldedNodesWithOpts(seed, dispatch, CircuitOpts{})
}

// ScaffoldedNodesWithOpts returns a scaffolded NodeRegistry with optional
// mechanical verification, self-verification scoring, and transcript recording.
func ScaffoldedNodesWithOpts(seed *Seed, dispatch ProbeDispatcher, opts CircuitOpts) engine.NodeRegistry {
	hintState := &hintTracker{
		hints:     seed.Hints,
		nextIndex: 0,
	}

	var tl time.Duration
	if seed.TimeLimit != "" {
		tl, _ = time.ParseDuration(seed.TimeLimit)
	}

	return engine.NodeRegistry{
		"ouroboros-generate": func(_ circuit.NodeDef) circuit.Node {
			return &generateNode{seed: seed, dispatch: dispatch, recorder: opts.Recorder}
		},
		"ouroboros-subject-scaffolded": func(_ circuit.NodeDef) circuit.Node {
			return &scaffoldedSubjectNode{
				dispatch:     dispatch,
				outputFormat: seed.OutputFormat,
				verifyHint:   seed.Verify != nil,
				timeLimit:    tl,
				recorder:     opts.Recorder,
			}
		},
		"ouroboros-judge-scaffolded": func(_ circuit.NodeDef) circuit.Node {
			return &scaffoldedJudgeNode{
				seed:       seed,
				dispatch:   dispatch,
				tracker:    hintState,
				verifier:   opts.Verifier,
				selfVerify: opts.SelfVerify,
				recorder:   opts.Recorder,
			}
		},
		"ouroboros-hint": func(_ circuit.NodeDef) circuit.Node {
			return &hintNode{tracker: hintState}
		},
	}
}

// ---------------------------------------------------------------------------
// Hint tracker — shared state between scaffolded judge and hint nodes
// ---------------------------------------------------------------------------

type hintTracker struct {
	hints     []string
	nextIndex int
}

func (ht *hintTracker) hasMore() bool {
	return ht.nextIndex < len(ht.hints)
}

func (ht *hintTracker) next() (int, string) {
	idx := ht.nextIndex
	hint := ht.hints[idx]
	ht.nextIndex++
	return idx, hint
}

func (ht *hintTracker) used() int {
	return ht.nextIndex
}

// ---------------------------------------------------------------------------
// Scaffolded Subject — accepts GeneratorOutput or HintFeedback
// ---------------------------------------------------------------------------

type scaffoldedSubjectNode struct {
	dispatch     ProbeDispatcher
	outputFormat string
	verifyHint   bool
	timeLimit    time.Duration
	recorder     TranscriptRecorder
}

func (n *scaffoldedSubjectNode) Name() string                       { return "subject" }
func (n *scaffoldedSubjectNode) ElementAffinity() circuit.Element { return circuit.ElementFire }

func (n *scaffoldedSubjectNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("scaffolded subject: no prior artifact")
	}

	var prompt string

	switch raw := nc.PriorArtifact.Raw().(type) {
	case *GeneratorOutput:
		prompt = raw.Question
	case *HintFeedback:
		prompt = fmt.Sprintf(`Your previous answer was insufficient.

Hint %d: %s

Original question:
%s

Your previous response:
%s

Please improve your answer using the hint above.`,
			raw.HintIndex+1, raw.Hint, raw.OriginalPrompt, raw.PriorResponse)
	default:
		return nil, fmt.Errorf("scaffolded subject: unexpected artifact type %T", nc.PriorArtifact.Raw())
	}

	if n.outputFormat != "" {
		prompt += "\n\nRespond in this exact format:\n" + n.outputFormat
	}
	if n.verifyHint {
		prompt += selfVerifyHint
	}

	dispatchCtx := ctx
	if n.timeLimit > 0 {
		var cancel context.CancelFunc
		dispatchCtx, cancel = context.WithTimeout(ctx, n.timeLimit)
		defer cancel()
	}

	start := time.Now()
	raw, err := n.dispatch(dispatchCtx, "subject", prompt)
	elapsed := time.Since(start)

	if n.recorder != nil && err == nil {
		n.recorder("subject", prompt, raw, elapsed)
	}

	timedOut := dispatchCtx.Err() == context.DeadlineExceeded
	if err != nil && !timedOut {
		return nil, fmt.Errorf("scaffolded subject dispatch: %w", err)
	}
	if timedOut && raw == "" {
		raw = "[timed out]"
	}

	return &seedArtifact{
		typeName:   "subject-response",
		confidence: 1.0,
		raw:        raw,
		metadata:   map[string]any{"timed_out": timedOut, "time_limit": n.timeLimit},
	}, nil
}

// ---------------------------------------------------------------------------
// Scaffolded Judge — evaluates response, emits hint or final pole result
// ---------------------------------------------------------------------------

type scaffoldedJudgeNode struct {
	seed       *Seed
	dispatch   ProbeDispatcher
	tracker    *hintTracker
	verifier   CodeVerifier
	selfVerify SelfVerifyScorer
	recorder   TranscriptRecorder
	lastPrompt string
}

func (n *scaffoldedJudgeNode) Name() string                       { return "judge" }
func (n *scaffoldedJudgeNode) ElementAffinity() agentport.Element { return agentport.ElementDiamond }

func (n *scaffoldedJudgeNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("scaffolded judge: no prior artifact (expected subject response)")
	}

	subjectResponse, ok := nc.PriorArtifact.Raw().(string)
	if !ok {
		return nil, fmt.Errorf("scaffolded judge: expected string, got %T", nc.PriorArtifact.Raw())
	}

	var mvr *MechanicalVerifyResult
	var verifyScores map[Dimension]float64
	if n.verifier != nil && n.seed.Verify != nil {
		mvr, verifyScores = n.verifier(subjectResponse, n.seed.Verify)
	}

	sufficient, err := n.assessSufficiency(ctx, subjectResponse, mvr)
	if err != nil {
		return nil, fmt.Errorf("scaffolded judge assess: %w", err)
	}

	if !sufficient && n.tracker.hasMore() {
		idx, hint := n.tracker.next()
		return &seedArtifact{
			typeName:   "hint-feedback",
			confidence: 0.5,
			raw: &HintFeedback{
				HintIndex:      idx,
				Hint:           hint,
				OriginalPrompt: n.lastPrompt,
				PriorResponse:  subjectResponse,
			},
		}, nil
	}

	prompt := buildJudgePrompt(n.seed, subjectResponse, mvr)
	start := time.Now()
	raw, err := n.dispatch(ctx, "judge", prompt)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("scaffolded judge dispatch: %w", err)
	}

	if n.recorder != nil {
		n.recorder("judge", prompt, raw, elapsed)
	}

	result, err := parseJudgeOutput(raw, n.seed)
	if err != nil {
		return nil, fmt.Errorf("scaffolded judge parse: %w", err)
	}

	result.GoldSignalScore = scoreGoldSignals(subjectResponse, n.seed, result.SelectedPole)
	result.HintsUsed = n.tracker.used()

	if mvr != nil {
		result.MechanicalVerify = mvr
		for dim, score := range verifyScores {
			if existing, ok := result.DimensionScores[dim]; ok {
				result.DimensionScores[dim] = (existing + score) / 2
			} else {
				result.DimensionScores[dim] = score
			}
		}
	}

	if n.selfVerify != nil {
		result.SelfVerifyScore = n.selfVerify(subjectResponse)
		applySlefVerifyAdjustments(result)
	}

	if sa, ok := nc.PriorArtifact.(*seedArtifact); ok && sa.metadata != nil {
		if to, ok := sa.metadata["timed_out"].(bool); ok && to {
			result.TimedOut = true
		}
		if tl, ok := sa.metadata["time_limit"].(time.Duration); ok {
			result.TimeLimit = tl
		}
	}

	return &seedArtifact{
		typeName:   "pole-result",
		confidence: result.Confidence,
		raw:        result,
	}, nil
}

func (n *scaffoldedJudgeNode) assessSufficiency(ctx context.Context, subjectResponse string, mvr *MechanicalVerifyResult) (bool, error) {
	if mvr != nil && !mvr.Compiled {
		return false, nil
	}

	prompt := fmt.Sprintf(`You are a behavioral assessment judge evaluating sufficiency.

Rubric: %s

The subject provided this response:
---
%s
---

Is this response sufficient to make a final pole assessment, or does the subject need a hint?
Respond with exactly one word: SUFFICIENT or INSUFFICIENT`,
		n.seed.Rubric,
		subjectResponse,
	)

	start := time.Now()
	raw, err := n.dispatch(ctx, "judge", prompt)
	elapsed := time.Since(start)
	if err != nil {
		return false, err
	}

	if n.recorder != nil {
		n.recorder("judge-assess", prompt, raw, elapsed)
	}

	n.lastPrompt = subjectResponse
	return raw == "SUFFICIENT" || len(raw) > 0 && raw[0] == 'S', nil
}

// ---------------------------------------------------------------------------
// Hint node — formats hint into a HintFeedback artifact for Subject
// ---------------------------------------------------------------------------

type hintNode struct {
	tracker *hintTracker
}

func (n *hintNode) Name() string                       { return "hint" }
func (n *hintNode) ElementAffinity() circuit.Element { return circuit.ElementWater }

func (n *hintNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("hint node: no prior artifact")
	}

	hf, ok := nc.PriorArtifact.Raw().(*HintFeedback)
	if !ok {
		return nil, fmt.Errorf("hint node: expected *HintFeedback, got %T", nc.PriorArtifact.Raw())
	}

	return &seedArtifact{
		typeName:   "hint-feedback",
		confidence: 1.0,
		raw:        hf,
	}, nil
}

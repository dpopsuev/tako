package ouroboros

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/bugle/element"
)

// JudgeFeedback is passed from Judge to Subject on non-final rounds.
type JudgeFeedback struct {
	Round          int    `json:"round"`
	TotalRounds    int    `json:"total_rounds"`
	Feedback       string `json:"feedback"`
	OriginalPrompt string `json:"original_prompt"`
}

// MultiRoundNodes returns a NodeRegistry for the multi-round probe circuit.
func MultiRoundNodes(seed *Seed, dispatch ProbeDispatcher) engine.NodeRegistry {
	return MultiRoundNodesWithOpts(seed, dispatch, CircuitOpts{})
}

// MultiRoundNodesWithOpts returns a multi-round NodeRegistry with optional
// mechanical verification and self-verification scoring.
func MultiRoundNodesWithOpts(seed *Seed, dispatch ProbeDispatcher, opts CircuitOpts) engine.NodeRegistry {
	roundState := &roundTracker{
		maxRounds:    seed.Rounds,
		currentRound: 0,
	}

	var tl time.Duration
	if seed.TimeLimit != "" {
		tl, _ = time.ParseDuration(seed.TimeLimit)
	}

	return engine.NodeRegistry{
		"ouroboros-generate": func(_ circuit.NodeDef) circuit.Node {
			return &generateNode{seed: seed, dispatch: dispatch, recorder: opts.Recorder}
		},
		"ouroboros-subject-multiround": func(_ circuit.NodeDef) circuit.Node {
			return &multiRoundSubjectNode{
				dispatch:     dispatch,
				outputFormat: seed.OutputFormat,
				verifyHint:   seed.Verify != nil,
				tracker:      roundState,
				timeLimit:    tl,
				recorder:     opts.Recorder,
			}
		},
		"ouroboros-judge-multiround": func(_ circuit.NodeDef) circuit.Node {
			return &multiRoundJudgeNode{
				seed:       seed,
				dispatch:   dispatch,
				tracker:    roundState,
				verifier:   opts.Verifier,
				selfVerify: opts.SelfVerify,
				recorder:   opts.Recorder,
			}
		},
	}
}

type roundTracker struct {
	maxRounds    int
	currentRound int
}

func (rt *roundTracker) isFinalRound() bool {
	return rt.currentRound >= rt.maxRounds
}

func (rt *roundTracker) advance() {
	rt.currentRound++
}

// ---------------------------------------------------------------------------
// Multi-round Subject
// ---------------------------------------------------------------------------

type multiRoundSubjectNode struct {
	dispatch     ProbeDispatcher
	outputFormat string
	verifyHint   bool
	tracker      *roundTracker
	timeLimit    time.Duration
	recorder     TranscriptRecorder
}

func (n *multiRoundSubjectNode) Name() string                       { return "subject" }
func (n *multiRoundSubjectNode) ElementAffinity() circuit.Element { return circuit.ElementFire }

func (n *multiRoundSubjectNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("multiround subject: no prior artifact")
	}

	var prompt string

	switch raw := nc.PriorArtifact.Raw().(type) {
	case *GeneratorOutput:
		prompt = raw.Question
	case *JudgeFeedback:
		prompt = fmt.Sprintf(`Previous feedback (round %d/%d):
%s

Original question:
%s

Please improve your answer based on the feedback above.`,
			raw.Round, raw.TotalRounds, raw.Feedback, raw.OriginalPrompt)
	default:
		return nil, fmt.Errorf("multiround subject: unexpected artifact type %T", nc.PriorArtifact.Raw())
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
	response, err := n.dispatch(dispatchCtx, "subject", prompt)
	elapsed := time.Since(start)

	if n.recorder != nil && err == nil {
		n.recorder("subject", prompt, response, elapsed)
	}

	timedOut := dispatchCtx.Err() == context.DeadlineExceeded
	if err != nil && !timedOut {
		return nil, fmt.Errorf("multiround subject dispatch: %w", err)
	}
	if timedOut && response == "" {
		response = "[timed out]"
	}

	return &seedArtifact{
		typeName:   "subject-response",
		confidence: 1.0,
		raw:        response,
		metadata:   map[string]any{"timed_out": timedOut, "time_limit": n.timeLimit},
	}, nil
}

// ---------------------------------------------------------------------------
// Multi-round Judge
// ---------------------------------------------------------------------------

type multiRoundJudgeNode struct {
	seed       *Seed
	dispatch   ProbeDispatcher
	tracker    *roundTracker
	verifier   CodeVerifier
	selfVerify SelfVerifyScorer
	recorder   TranscriptRecorder
}

func (n *multiRoundJudgeNode) Name() string                       { return "judge" }
func (n *multiRoundJudgeNode) ElementAffinity() element.Element { return element.ElementDiamond }

func (n *multiRoundJudgeNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("multiround judge: no prior artifact (expected subject response)")
	}

	subjectResponse, ok := nc.PriorArtifact.Raw().(string)
	if !ok {
		return nil, fmt.Errorf("multiround judge: expected string, got %T", nc.PriorArtifact.Raw())
	}

	n.tracker.advance()

	// Run mechanical verification on every round — even non-final.
	// On non-final rounds, compile/test errors become feedback.
	var mvr *MechanicalVerifyResult
	var verifyScores map[Dimension]float64
	if n.verifier != nil && n.seed.Verify != nil {
		mvr, verifyScores = n.verifier(subjectResponse, n.seed.Verify)
	}

	if !n.tracker.isFinalRound() {
		feedback, err := n.generateFeedbackWithVerify(ctx, subjectResponse, mvr)
		if err != nil {
			return nil, fmt.Errorf("multiround judge feedback: %w", err)
		}

		return &seedArtifact{
			typeName:   "judge-feedback",
			confidence: 0.5,
			raw: &JudgeFeedback{
				Round:          n.tracker.currentRound,
				TotalRounds:    n.tracker.maxRounds,
				Feedback:       feedback,
				OriginalPrompt: subjectResponse,
			},
		}, nil
	}

	prompt := buildJudgePrompt(n.seed, subjectResponse, mvr)
	start := time.Now()
	raw, err := n.dispatch(ctx, "judge", prompt)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("multiround judge dispatch: %w", err)
	}

	if n.recorder != nil {
		n.recorder("judge", prompt, raw, elapsed)
	}

	result, err := parseJudgeOutput(raw, n.seed)
	if err != nil {
		return nil, fmt.Errorf("multiround judge parse: %w", err)
	}

	result.GoldSignalScore = scoreGoldSignals(subjectResponse, n.seed, result.SelectedPole)

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

func (n *multiRoundJudgeNode) generateFeedbackWithVerify(ctx context.Context, subjectResponse string, mvr *MechanicalVerifyResult) (string, error) {
	var verifyFeedback string
	if mvr != nil {
		var sb strings.Builder
		sb.WriteString("\nMechanical verification results for the subject's code:\n")
		if !mvr.Compiled {
			sb.WriteString(fmt.Sprintf("- COMPILATION FAILED: %s\n", mvr.CompileErr))
			sb.WriteString("The code does not compile. Include specific compile errors in your feedback.\n")
		} else if !mvr.TestsPassed {
			sb.WriteString("- Compilation: PASSED\n")
			sb.WriteString(fmt.Sprintf("- TESTS FAILED: %s\n", mvr.TestErr))
			sb.WriteString("The code compiles but tests fail. Include specific test failures in your feedback.\n")
		} else {
			sb.WriteString("- Compilation: PASSED\n")
			sb.WriteString("- Tests: PASSED\n")
			if mvr.BenchmarkMs > 0 && !mvr.BenchmarkPassed {
				sb.WriteString(fmt.Sprintf("- PERFORMANCE FAILED: %s\n", mvr.BenchmarkErr))
				sb.WriteString("Code is correct but slow. Focus feedback on optimization.\n")
			}
		}
		verifyFeedback = sb.String()
	}

	prompt := fmt.Sprintf(`You are a behavioral assessment judge providing feedback.

Rubric: %s
%s
The subject provided this response:
---
%s
---

This is round %d of %d. Provide constructive feedback to help the subject improve.
Focus on what is missing or could be deeper. Be specific.
If mechanical verification results are provided, reference them directly.

Format:
FEEDBACK: <your feedback>`,
		n.seed.Rubric,
		verifyFeedback,
		subjectResponse,
		n.tracker.currentRound,
		n.tracker.maxRounds,
	)

	raw, err := n.dispatch(ctx, "judge", prompt)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if f, ok := strings.CutPrefix(trimmed, "FEEDBACK:"); ok {
			return strings.TrimSpace(f), nil
		}
	}

	return raw, nil
}


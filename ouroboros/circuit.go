package ouroboros

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/bugle/element"
)

// sortedPoleNames returns the pole names from a seed in deterministic
// (alphabetical) order. Map iteration order is non-deterministic in Go;
// this ensures prompts and parsers see poles in a stable order.
func sortedPoleNames(seed *Seed) []string {
	names := make([]string, 0, len(seed.Poles))
	for name := range seed.Poles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProbeDispatcher sends a prompt to an LLM and returns the raw response.
// Used by the 3-node seed circuit (generate, subject, judge).
//
// Design note: parseGeneratorOutput and parseJudgeOutput are intentionally
// NOT framework Extractors. They require *Seed context (pole names, rubric)
// that the Extractor interface (Name + Extract(ctx, any)) cannot provide.
// These parsers belong inside Node.Process where the seed is available.
type ProbeDispatcher func(ctx context.Context, nodeName string, prompt string) (string, error)

// CodeVerifier runs mechanical verification on a subject's code response.
// Injected into the Judge to avoid a circular dependency (ouroboros → probes).
// Returns a MechanicalVerifyResult and dimension score adjustments.
type CodeVerifier func(response string, verify *SeedVerify) (*MechanicalVerifyResult, map[Dimension]float64)

// SelfVerifyScorer analyzes a subject response for self-verification behavior
// (test cases, edge case handling, self-correction). Returns 0.0-1.0.
type SelfVerifyScorer func(response string) float64

// seedArtifact wraps a typed output as an Artifact for the seed circuit.
type seedArtifact struct {
	typeName   string
	confidence float64
	raw        any
	metadata   map[string]any
}

func (a *seedArtifact) Type() string       { return a.typeName }
func (a *seedArtifact) Confidence() float64 { return a.confidence }
func (a *seedArtifact) Raw() any            { return a.raw }
func (a *seedArtifact) Meta() map[string]any { return a.metadata }

// TranscriptRecorder captures each exchange during a probe walk.
// Called by node Process methods after each dispatch call.
type TranscriptRecorder func(role string, prompt string, response string, elapsed time.Duration)

// CircuitOpts bundles optional callbacks for the probe circuit.
// All fields are optional — nil means the feature is disabled.
type CircuitOpts struct {
	Verifier   CodeVerifier
	SelfVerify SelfVerifyScorer
	Recorder   TranscriptRecorder
}

// CircuitNodes returns a NodeRegistry for the ouroboros-probe circuit.
// Each node is constructed with the seed and dispatcher it needs.
func CircuitNodes(seed *Seed, dispatch ProbeDispatcher) engine.NodeRegistry {
	return CircuitNodesWithOpts(seed, dispatch, CircuitOpts{})
}

// CircuitNodesWithOpts returns a NodeRegistry with optional mechanical
// verification and self-verification scoring injected into the Judge.
func CircuitNodesWithOpts(seed *Seed, dispatch ProbeDispatcher, opts CircuitOpts) engine.NodeRegistry {
	var tl time.Duration
	if seed.TimeLimit != "" {
		tl, _ = time.ParseDuration(seed.TimeLimit)
	}

	return engine.NodeRegistry{
		"ouroboros-generate": func(_ circuit.NodeDef) circuit.Node {
			return &generateNode{seed: seed, dispatch: dispatch, recorder: opts.Recorder}
		},
		"ouroboros-subject": func(_ circuit.NodeDef) circuit.Node {
			return &subjectNode{
				dispatch:     dispatch,
				outputFormat: seed.OutputFormat,
				verifyHint:   seed.Verify != nil,
				timeLimit:    tl,
				recorder:     opts.Recorder,
			}
		},
		"ouroboros-judge": func(_ circuit.NodeDef) circuit.Node {
			return &judgeNode{
				seed:       seed,
				dispatch:   dispatch,
				verifier:   opts.Verifier,
				selfVerify: opts.SelfVerify,
				recorder:   opts.Recorder,
			}
		},
	}
}

// ---------------------------------------------------------------------------
// Generate node (thesis)
// ---------------------------------------------------------------------------

type generateNode struct {
	seed     *Seed
	dispatch ProbeDispatcher
	recorder TranscriptRecorder
}

func (n *generateNode) Name() string                       { return "generate" }
func (n *generateNode) ElementAffinity() element.Element { return element.ElementEarth }

func (n *generateNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	prompt := n.buildPrompt()
	start := time.Now()
	raw, err := n.dispatch(ctx, "generate", prompt)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("generate dispatch: %w", err)
	}

	if n.recorder != nil {
		n.recorder("generate", prompt, raw, elapsed)
	}

	output, err := parseGeneratorOutput(raw, n.seed)
	if err != nil {
		return nil, fmt.Errorf("generate parse: %w", err)
	}

	return &seedArtifact{
		typeName:   "generator-output",
		confidence: 1.0,
		raw:        output,
	}, nil
}

func (n *generateNode) buildPrompt() string {
	poleNames := sortedPoleNames(n.seed)

	return fmt.Sprintf(`You are a behavioral assessment question generator.

Context: %s

Instructions: %s

Create a realistic scenario question based on the context above.
Also provide two reference answers — one for each behavioral pole:
- Pole "%s": %s
- Pole "%s": %s

Respond in this exact format:
QUESTION: <your question>
ANSWER_%s: <ideal answer for this pole>
ANSWER_%s: <ideal answer for this pole>`,
		n.seed.Context,
		n.seed.GeneratorInstructions,
		poleNames[0], n.seed.Poles[poleNames[0]].Signal,
		poleNames[1], n.seed.Poles[poleNames[1]].Signal,
		poleNames[0],
		poleNames[1],
	)
}

// parseGeneratorOutput extracts question and pole answers from the LLM response.
// Expected format:
//
//	QUESTION: <question text>
//	ANSWER_<pole>: <answer text>
//
// Falls back to using the raw response as the question and seed signals
// as pole answers when structured parsing yields nothing.
func parseGeneratorOutput(raw string, seed *Seed) (*GeneratorOutput, error) {
	output := &GeneratorOutput{
		PoleAnswers: make(map[string]string),
	}

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if q, ok := strings.CutPrefix(trimmed, "QUESTION:"); ok {
			output.Question = strings.TrimSpace(q)
			continue
		}
		for name := range seed.Poles {
			prefix := fmt.Sprintf("ANSWER_%s:", name)
			if a, ok := strings.CutPrefix(trimmed, prefix); ok {
				output.PoleAnswers[name] = strings.TrimSpace(a)
			}
		}
	}

	if output.Question == "" {
		output.Question = raw
	}
	for name := range seed.Poles {
		if _, ok := output.PoleAnswers[name]; !ok {
			output.PoleAnswers[name] = seed.Poles[name].Signal
		}
	}

	return output, nil
}

// ---------------------------------------------------------------------------
// Subject node (antithesis) — receives ONLY the question, no rubric or poles
// ---------------------------------------------------------------------------

type subjectNode struct {
	dispatch     ProbeDispatcher
	outputFormat string
	verifyHint   bool
	timeLimit    time.Duration
	recorder     TranscriptRecorder
}

func (n *subjectNode) Name() string                       { return "subject" }
func (n *subjectNode) ElementAffinity() element.Element { return element.ElementFire }

func (n *subjectNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("subject node: no prior artifact (expected generator output)")
	}

	genOutput, ok := nc.PriorArtifact.Raw().(*GeneratorOutput)
	if !ok {
		return nil, fmt.Errorf("subject node: expected *GeneratorOutput, got %T", nc.PriorArtifact.Raw())
	}

	prompt := genOutput.Question
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
		return nil, fmt.Errorf("subject dispatch: %w", err)
	}
	if timedOut {
		if raw == "" {
			raw = "[timed out]"
		}
	}

	return &seedArtifact{
		typeName:   "subject-response",
		confidence: 1.0,
		raw:        raw,
		metadata:   map[string]any{"timed_out": timedOut, "time_limit": n.timeLimit},
	}, nil
}

const selfVerifyHint = `

Before submitting, verify your work:
- Does your code compile without errors?
- Have you considered edge cases?
- Would your solution pass basic tests?
- Are there limitations you should acknowledge?`

// ---------------------------------------------------------------------------
// Judge node (synthesis) — classifies which pole the subject's answer aligns with
// ---------------------------------------------------------------------------

type judgeNode struct {
	seed       *Seed
	dispatch   ProbeDispatcher
	verifier   CodeVerifier
	selfVerify SelfVerifyScorer
	recorder   TranscriptRecorder
}

func (n *judgeNode) Name() string                       { return "judge" }
func (n *judgeNode) ElementAffinity() element.Element { return element.ElementDiamond }

func (n *judgeNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if nc.PriorArtifact == nil {
		return nil, fmt.Errorf("judge node: no prior artifact (expected subject response)")
	}

	subjectResponse, ok := nc.PriorArtifact.Raw().(string)
	if !ok {
		return nil, fmt.Errorf("judge node: expected string, got %T", nc.PriorArtifact.Raw())
	}

	// Run mechanical verification BEFORE the LLM judge so we can
	// include the compile/test results in the judge prompt.
	var mvr *MechanicalVerifyResult
	var verifyScores map[Dimension]float64
	if n.verifier != nil && n.seed.Verify != nil {
		mvr, verifyScores = n.verifier(subjectResponse, n.seed.Verify)
	}

	prompt := buildJudgePrompt(n.seed, subjectResponse, mvr)
	start := time.Now()
	raw, err := n.dispatch(ctx, "judge", prompt)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("judge dispatch: %w", err)
	}

	if n.recorder != nil {
		n.recorder("judge", prompt, raw, elapsed)
	}

	result, err := parseJudgeOutput(raw, n.seed)
	if err != nil {
		return nil, fmt.Errorf("judge parse: %w", err)
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

// applySlefVerifyAdjustments rewards self-verification behavior by adjusting
// dimension scores. High self-verify → boost convergence/persistence/evidence,
// penalize shortcut_affinity.
func applySlefVerifyAdjustments(result *PoleResult) {
	sv := result.SelfVerifyScore
	if sv <= 0 {
		return
	}

	boost := sv * 0.15
	for _, dim := range []Dimension{DimConvergenceThreshold, DimPersistence, DimEvidenceDepth} {
		if v, ok := result.DimensionScores[dim]; ok {
			result.DimensionScores[dim] = clampDim(v + boost)
		}
	}
	if v, ok := result.DimensionScores[DimShortcutAffinity]; ok {
		result.DimensionScores[DimShortcutAffinity] = clampDim(v - boost)
	}
}

func clampDim(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// buildJudgePrompt constructs the LLM prompt for the Judge. Shared by
// cascade and multiround judges. When verifyResult is non-nil, the
// mechanical verification outcome is included in the prompt so the LLM
// judge can incorporate it into its reasoning.
func buildJudgePrompt(seed *Seed, subjectResponse string, verifyResult *MechanicalVerifyResult) string {
	poleNames := sortedPoleNames(seed)

	var goldSection string
	if seed.GoldAnswer != "" {
		goldSection = fmt.Sprintf("\nGold reference answer:\n---\n%s---\n", seed.GoldAnswer)
	}

	var signalSection string
	if len(seed.GoldSignals) > 0 {
		var sb strings.Builder
		sb.WriteString("\nExpected signals per pole:\n")
		for _, name := range poleNames {
			if signals, ok := seed.GoldSignals[name]; ok {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", name, strings.Join(signals, ", ")))
			}
		}
		signalSection = sb.String()
	}

	var verifySection string
	if verifyResult != nil {
		var sb strings.Builder
		sb.WriteString("\nMechanical verification results:\n")
		if verifyResult.Compiled {
			sb.WriteString("- Compilation: PASSED\n")
		} else {
			sb.WriteString(fmt.Sprintf("- Compilation: FAILED (%s)\n", verifyResult.CompileErr))
		}
		if verifyResult.TestsPassed {
			sb.WriteString("- Tests: PASSED\n")
		} else if verifyResult.TestErr != "" {
			sb.WriteString(fmt.Sprintf("- Tests: FAILED (%s)\n", verifyResult.TestErr))
		}
		if verifyResult.BenchmarkMs > 0 {
			if verifyResult.BenchmarkPassed {
				sb.WriteString(fmt.Sprintf("- Performance: PASSED (%dms)\n", verifyResult.BenchmarkMs))
			} else {
				sb.WriteString(fmt.Sprintf("- Performance: FAILED (%s)\n", verifyResult.BenchmarkErr))
			}
		}
		sb.WriteString("Factor these mechanical results into your confidence score.\n")
		verifySection = sb.String()
	}

	return fmt.Sprintf(`You are a behavioral classification judge.

Rubric: %s
%s%s%s
The subject was given a task and responded. Classify which behavioral pole
the response aligns with. If a gold reference is provided, also assess
correctness — does the response cover the key points from the reference?

Pole "%s": %s
Pole "%s": %s

Subject's response:
---
%s
---

Respond in this exact format:
SELECTED_POLE: <pole name>
CONFIDENCE: <0.0 to 1.0>
REASONING: <brief explanation>`,
		seed.Rubric,
		goldSection,
		signalSection,
		verifySection,
		poleNames[0], seed.Poles[poleNames[0]].Signal,
		poleNames[1], seed.Poles[poleNames[1]].Signal,
		subjectResponse,
	)
}

// parseJudgeOutput extracts PoleResult from the LLM response.
// Expected format:
//
//	SELECTED_POLE: <pole name>
//	CONFIDENCE: <0.0 to 1.0>
//	REASONING: <brief explanation>
//
// Falls back to substring search for pole names and confidence 0.8 when
// structured parsing fails.
func parseJudgeOutput(raw string, seed *Seed) (*PoleResult, error) {
	poleNames := sortedPoleNames(seed)

	var selectedPole string
	confidence := -1.0
	var reasoning string

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if p, ok := strings.CutPrefix(trimmed, "SELECTED_POLE:"); ok {
			selectedPole = strings.TrimSpace(p)
			continue
		}
		if c, ok := strings.CutPrefix(trimmed, "CONFIDENCE:"); ok {
			if v, err := strconv.ParseFloat(strings.TrimSpace(c), 64); err == nil {
				confidence = v
			}
			continue
		}
		if r, ok := strings.CutPrefix(trimmed, "REASONING:"); ok {
			reasoning = strings.TrimSpace(r)
			continue
		}
	}

	if _, ok := seed.Poles[selectedPole]; !ok {
		selectedPole = poleNames[0]
		for _, name := range poleNames {
			if strings.Contains(raw, name) {
				selectedPole = name
				break
			}
		}
	}
	if confidence < 0 || confidence > 1 {
		confidence = 0.8
	}
	if reasoning == "" {
		reasoning = raw
	}

	pole := seed.Poles[selectedPole]
	scores := make(map[Dimension]float64, len(pole.ElementAffinity))
	for dim, score := range pole.ElementAffinity {
		scores[dim] = score
	}

	return &PoleResult{
		SelectedPole:    selectedPole,
		Confidence:      confidence,
		DimensionScores: scores,
		Reasoning:       reasoning,
	}, nil
}

// scoreGoldSignals computes the fraction of gold signals present in the
// subject's response for the selected pole. Returns 0.0 if no gold
// signals are defined.
func scoreGoldSignals(subjectResponse string, seed *Seed, selectedPole string) float64 {
	if len(seed.GoldSignals) == 0 {
		return 0.0
	}
	signals, ok := seed.GoldSignals[selectedPole]
	if !ok || len(signals) == 0 {
		return 0.0
	}
	lower := strings.ToLower(subjectResponse)
	matched := 0
	for _, sig := range signals {
		if strings.Contains(lower, strings.ToLower(sig)) {
			matched++
		}
	}
	return float64(matched) / float64(len(signals))
}

package ouroboros

import (
	"context"
	"fmt"
	"os"
	"testing"

	framework "github.com/dpopsuev/origami"
	_ "github.com/dpopsuev/origami/topology"
)

func testSeed() *Seed {
	return &Seed{
		Name:    "test-probe",
		Version: "1.0",
		Dimensions: []Dimension{DimSpeed, DimEvidenceDepth},
		Category: CategorySkill,
		Poles: map[string]Pole{
			"systematic": {
				Signal: "thorough, step-by-step analysis",
				ElementAffinity: map[Dimension]float64{
					DimSpeed:         0.3,
					DimEvidenceDepth: 0.9,
				},
			},
			"heuristic": {
				Signal: "pattern matching, quick answer",
				ElementAffinity: map[Dimension]float64{
					DimSpeed:         0.9,
					DimEvidenceDepth: 0.3,
				},
			},
		},
		Context:               "You are reviewing a complex Go function with subtle concurrency bugs.",
		Rubric:                "Evaluate whether the response uses systematic analysis or heuristic shortcuts.",
		GeneratorInstructions: "Create a realistic code review scenario with a goroutine leak.",
	}
}

func stubProbeDispatcher(responses map[string]string) ProbeDispatcher {
	return func(ctx context.Context, nodeName string, prompt string) (string, error) {
		resp, ok := responses[nodeName]
		if !ok {
			return fmt.Sprintf("stub response for %s", nodeName), nil
		}
		return resp, nil
	}
}

func TestCircuitWalk_FullFlow(t *testing.T) {
	seed := testSeed()
	dispatcher := stubProbeDispatcher(map[string]string{
		"generate": "QUESTION: Review this goroutine leak\nANSWER_systematic: Analyze the context cancellation path\nANSWER_heuristic: Just add a defer cancel()",
		"subject":  "I would carefully trace the goroutine lifecycle, check context propagation, and verify all channels are properly closed. This is a systematic approach.",
		"judge":    "SELECTED_POLE: systematic\nCONFIDENCE: 0.85\nREASONING: The response shows thorough analysis.",
	})

	circuitYAML, err := os.ReadFile("circuits/ouroboros-probe.yaml")
	if err != nil {
		t.Fatalf("read circuit YAML: %v", err)
	}
	def, err := framework.LoadCircuit(circuitYAML)
	if err != nil {
		t.Fatalf("load circuit: %v", err)
	}

	nodes := CircuitNodes(seed, dispatcher)
	g, err := framework.BuildGraph(def, framework.GraphRegistries{Nodes: nodes})
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	walker := framework.NewProcessWalker("test-walk")
	ctx := context.Background()

	if err := g.Walk(ctx, walker, def.Start); err != nil {
		t.Fatalf("walk failed: %v", err)
	}

	if walker.State().Status != "done" {
		t.Errorf("walker status = %q, want done", walker.State().Status)
	}

	history := walker.State().History
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3 (generate, subject, judge)", len(history))
	}
	if history[0].Node != "generate" {
		t.Errorf("history[0].Node = %q, want generate", history[0].Node)
	}
	if history[1].Node != "subject" {
		t.Errorf("history[1].Node = %q, want subject", history[1].Node)
	}
	if history[2].Node != "judge" {
		t.Errorf("history[2].Node = %q, want judge", history[2].Node)
	}

	judgeArtifact := walker.State().Outputs["judge"]
	if judgeArtifact == nil {
		t.Fatal("judge artifact is nil")
	}
	result, ok := judgeArtifact.Raw().(*PoleResult)
	if !ok {
		t.Fatalf("judge artifact raw type = %T, want *PoleResult", judgeArtifact.Raw())
	}
	if result.SelectedPole != "systematic" {
		t.Errorf("selected pole = %q, want systematic", result.SelectedPole)
	}
	if result.Confidence != 0.85 {
		t.Errorf("confidence = %v, want 0.85 (parsed from response)", result.Confidence)
	}
	if result.Reasoning != "The response shows thorough analysis." {
		t.Errorf("reasoning = %q, want parsed reasoning", result.Reasoning)
	}
	if len(result.DimensionScores) == 0 {
		t.Error("dimension scores are empty")
	}
	if result.DimensionScores[DimEvidenceDepth] != 0.9 {
		t.Errorf("evidence_depth score = %v, want 0.9", result.DimensionScores[DimEvidenceDepth])
	}
}

func TestCircuitNodes_AllRegistered(t *testing.T) {
	seed := testSeed()
	nodes := CircuitNodes(seed, stubProbeDispatcher(nil))

	for _, family := range []string{"ouroboros-generate", "ouroboros-subject", "ouroboros-judge"} {
		factory, ok := nodes[family]
		if !ok {
			t.Errorf("missing node factory for family %q", family)
			continue
		}
		node := factory(framework.NodeDef{})
		if node == nil {
			t.Errorf("factory for %q returned nil", family)
		}
	}
}

func TestSubjectNode_OnlySeesQuestion(t *testing.T) {
	var capturedPrompt string
	dispatcher := func(ctx context.Context, nodeName string, prompt string) (string, error) {
		if nodeName == "subject" {
			capturedPrompt = prompt
		}
		return "stub response", nil
	}

	node := &subjectNode{dispatch: dispatcher}

	genOutput := &GeneratorOutput{
		Question: "What would you do about this goroutine leak?",
		PoleAnswers: map[string]string{
			"systematic": "Trace the lifecycle",
			"heuristic":  "Add defer cancel()",
		},
	}

	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{
			typeName:   "generator-output",
			confidence: 1.0,
			raw:        genOutput,
		},
	}

	_, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("subject Process: %v", err)
	}

	if capturedPrompt != genOutput.Question {
		t.Errorf("subject received more than the question:\ngot:  %q\nwant: %q", capturedPrompt, genOutput.Question)
	}
}

func TestJudgeNode_ProducesPoleResult(t *testing.T) {
	s := testSeed()
	dispatcher := stubProbeDispatcher(map[string]string{
		"judge": "SELECTED_POLE: heuristic\nCONFIDENCE: 0.7\nREASONING: Quick answer pattern",
	})

	node := &judgeNode{seed: s, dispatch: dispatcher}
	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{
			typeName:   "subject-response",
			confidence: 1.0,
			raw:        "I'd just add defer cancel() and move on",
		},
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("judge Process: %v", err)
	}

	result, ok := artifact.Raw().(*PoleResult)
	if !ok {
		t.Fatalf("judge artifact raw = %T, want *PoleResult", artifact.Raw())
	}

	if result.SelectedPole != "heuristic" {
		t.Errorf("selected pole = %q, want heuristic", result.SelectedPole)
	}
	if result.Confidence != 0.7 {
		t.Errorf("confidence = %v, want 0.7 (parsed from response)", result.Confidence)
	}
	if result.Reasoning != "Quick answer pattern" {
		t.Errorf("reasoning = %q, want parsed reasoning", result.Reasoning)
	}
	if result.DimensionScores[DimSpeed] != 0.9 {
		t.Errorf("speed score = %v, want 0.9", result.DimensionScores[DimSpeed])
	}
}

func TestJudgeNode_MechanicalVerify(t *testing.T) {
	s := testSeed()
	s.Verify = &SeedVerify{Language: "go", Compile: "go build ./...", Test: "go test ./..."}

	dispatcher := stubProbeDispatcher(map[string]string{
		"judge": "SELECTED_POLE: systematic\nCONFIDENCE: 0.9\nREASONING: Thorough analysis with tests",
	})

	verifier := func(response string, verify *SeedVerify) (*MechanicalVerifyResult, map[Dimension]float64) {
		return &MechanicalVerifyResult{
			Compiled:    true,
			TestsPassed: true,
			Score:       0.7,
		}, map[Dimension]float64{
			DimEvidenceDepth: 0.7,
		}
	}

	node := &judgeNode{
		seed:     s,
		dispatch: dispatcher,
		verifier: verifier,
	}
	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{
			typeName:   "subject-response",
			confidence: 1.0,
			raw:        "```go\nfunc hello() {}\n```",
		},
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("judge Process: %v", err)
	}

	result := artifact.Raw().(*PoleResult)
	if result.MechanicalVerify == nil {
		t.Fatal("MechanicalVerify is nil, expected verify result")
	}
	if !result.MechanicalVerify.Compiled {
		t.Error("expected Compiled=true")
	}
	if !result.MechanicalVerify.TestsPassed {
		t.Error("expected TestsPassed=true")
	}
	if result.DimensionScores[DimEvidenceDepth] != 0.8 {
		t.Errorf("evidence_depth = %v, want 0.8 (avg of 0.9 pole + 0.7 verify)",
			result.DimensionScores[DimEvidenceDepth])
	}
}

func TestJudgeNode_SelfVerify(t *testing.T) {
	s := testSeed()
	dispatcher := stubProbeDispatcher(map[string]string{
		"judge": "SELECTED_POLE: systematic\nCONFIDENCE: 0.85\nREASONING: Self-checking",
	})

	node := &judgeNode{
		seed:     s,
		dispatch: dispatcher,
		selfVerify: func(response string) float64 {
			return 0.6
		},
	}
	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{
			typeName:   "subject-response",
			confidence: 1.0,
			raw:        "Let me verify this. func TestAdd(t *testing.T) { ... }",
		},
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("judge Process: %v", err)
	}

	result := artifact.Raw().(*PoleResult)
	if result.SelfVerifyScore != 0.6 {
		t.Errorf("SelfVerifyScore = %v, want 0.6", result.SelfVerifyScore)
	}
}

func TestCircuitNodesWithOpts_InjectsCallbacks(t *testing.T) {
	s := testSeed()
	s.Verify = &SeedVerify{Language: "go"}

	verifyCalled := false
	selfVerifyCalled := false
	opts := CircuitOpts{
		Verifier: func(resp string, v *SeedVerify) (*MechanicalVerifyResult, map[Dimension]float64) {
			verifyCalled = true
			return &MechanicalVerifyResult{Compiled: true, TestsPassed: true, Score: 0.7}, nil
		},
		SelfVerify: func(resp string) float64 {
			selfVerifyCalled = true
			return 0.5
		},
	}

	nodes := CircuitNodesWithOpts(s, stubProbeDispatcher(map[string]string{
		"judge": "SELECTED_POLE: systematic\nCONFIDENCE: 0.9\nREASONING: ok",
	}), opts)

	judgeFactory := nodes["ouroboros-judge"]
	judge := judgeFactory(framework.NodeDef{})

	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{typeName: "subject-response", confidence: 1.0, raw: "code here"},
	}
	_, err := judge.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("judge Process: %v", err)
	}

	if !verifyCalled {
		t.Error("verifier was not called")
	}
	if !selfVerifyCalled {
		t.Error("selfVerify was not called")
	}
}

func TestSubjectNode_VerifyHint(t *testing.T) {
	var capturedPrompt string
	dispatcher := func(ctx context.Context, nodeName string, prompt string) (string, error) {
		if nodeName == "subject" {
			capturedPrompt = prompt
		}
		return "stub response", nil
	}

	node := &subjectNode{dispatch: dispatcher, verifyHint: true}
	genOutput := &GeneratorOutput{
		Question:    "Fix this code.",
		PoleAnswers: map[string]string{"a": "x", "b": "y"},
	}

	nc := framework.NodeContext{
		PriorArtifact: &seedArtifact{typeName: "generator-output", confidence: 1.0, raw: genOutput},
	}
	_, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(capturedPrompt, "verify your work") {
		t.Errorf("expected self-verify hint in prompt, got:\n%s", capturedPrompt)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}
func containsImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

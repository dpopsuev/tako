package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

const testCircuitYAML = `
circuit: test-run
nodes:
  - name: start
    approach: fire
    instrument: transformer
    action: echo
  - name: finish
    approach: water
    instrument: transformer
    action: echo
edges:
  - id: E1
    name: go
    from: start
    to: finish
    when: "true"
  - id: E2
    name: done
    from: finish
    to: _done
    when: "true"
start: start
done: _done
`

func writeTempCircuit(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_BasicCircuit(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	trans := &echoTransformer{}

	err := Run(context.Background(), path, map[string]any{"data": true},
		WithTransformers(TransformerRegistry{"echo": trans}),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRun_WithOverrides(t *testing.T) {
	yaml := `
circuit: test-vars
vars:
  threshold: 0.5
nodes:
  - name: a
    approach: fire
    instrument: transformer
    action: echo
edges:
  - id: E1
    from: a
    to: _done
    when: "config.threshold > 0.8"
start: a
done: _done
`
	path := writeTempCircuit(t, yaml)
	trans := &echoTransformer{}

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": trans}),
		WithOverrides(map[string]any{"threshold": 0.9}),
	)
	if err != nil {
		t.Fatalf("Run with overrides: %v", err)
	}
}

func TestRun_WithHooks(t *testing.T) {
	yaml := `
circuit: test-hooks
nodes:
  - name: a
    approach: fire
    instrument: transformer
    action: echo
    after: [my-hook]
edges:
  - id: E1
    from: a
    to: _done
    when: "true"
start: a
done: _done
`
	path := writeTempCircuit(t, yaml)
	trans := &echoTransformer{}
	called := false
	hooks := HookRegistry{}
	hooks.Register(NewHookFunc("my-hook", func(_ context.Context, _ string, _ circuit.Artifact) error {
		called = true
		return nil
	}))

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": trans}),
		WithHooks(hooks),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !called {
		t.Error("hook was not called")
	}
}

func TestRun_MissingFile(t *testing.T) {
	err := Run(context.Background(), "/nonexistent/circuit.yaml", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRun_InvalidYAML(t *testing.T) {
	path := writeTempCircuit(t, "{{invalid yaml")
	err := Run(context.Background(), path, nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_ValidCircuit(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	err := Validate(path,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
	)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_InvalidExpression(t *testing.T) {
	yaml := `
circuit: bad
nodes:
  - name: a
    approach: fire
    instrument: transformer
    action: echo
edges:
  - id: E1
    from: a
    to: _done
    when: ">>> invalid"
start: a
done: _done
`
	path := writeTempCircuit(t, yaml)
	err := Validate(path, WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}))
	if err == nil {
		t.Fatal("expected validation error for invalid expression")
	}
}

func TestRun_InputResolutionAndPromptRendering(t *testing.T) {
	yaml := `
circuit: test-input-resolve
vars:
  threshold: 0.85
nodes:
  - name: recall
    approach: fire
    instrument: transformer
    action: echo
  - name: triage
    approach: water
    instrument: transformer
    action: capture
    input: "${recall.output}"
    prompt: "Node {{.Node}} sees threshold {{.Config.threshold}}"
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
  - id: E2
    from: triage
    to: _done
    when: "true"
start: recall
done: _done
`
	path := writeTempCircuit(t, yaml)

	var capturedPrompt string
	var capturedInput any

	capture := TransformerFunc("capture", func(_ context.Context, tc *TransformerContext) (any, error) {
		capturedPrompt = tc.Prompt
		capturedInput = tc.Input
		return map[string]any{"captured": true}, nil
	})

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{
			"echo":    &echoTransformer{},
			"capture": capture,
		}),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if capturedPrompt != "Node triage sees threshold 0.85" {
		t.Errorf("rendered prompt = %q", capturedPrompt)
	}

	inputMap, ok := capturedInput.(map[string]any)
	if !ok {
		t.Fatalf("input type = %T, want map from recall echo", capturedInput)
	}
	if inputMap["node"] != "recall" {
		t.Errorf("input should come from recall node, got node=%v", inputMap["node"])
	}
}

func TestRun_WithTeam_TwoWalkers(t *testing.T) {
	yaml := `
circuit: test-team
nodes:
  - name: classify
    approach: fire
    instrument: transformer
    action: echo
  - name: investigate
    approach: water
    instrument: transformer
    action: echo
edges:
  - id: E1
    from: classify
    to: investigate
    when: "true"
  - id: E2
    from: investigate
    to: _done
    when: "true"
start: classify
done: _done
`
	path := writeTempCircuit(t, yaml)

	herald := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "Herald",
			Element:      roster.ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.1},
		},
		state: circuit.NewWalkerState("herald-1"),
	}
	seeker := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "Seeker",
			Element:      roster.ElementWater,
			StepAffinity: map[string]float64{"classify": 0.1, "investigate": 0.9},
		},
		state: circuit.NewWalkerState("seeker-1"),
	}

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithCollective([]circuit.Walker{herald, seeker}, &AffinitySelector{}, WithMaxSteps(20)),
	)
	if err != nil {
		t.Fatalf("Run with team: %v", err)
	}

	if len(herald.visited) == 0 && len(seeker.visited) == 0 {
		t.Fatal("neither walker visited any nodes")
	}

	allVisited := make([]string, 0, len(herald.visited)+len(seeker.visited))
	allVisited = append(allVisited, herald.visited...)
	allVisited = append(allVisited, seeker.visited...)
	hasClassify, hasInvestigate := false, false
	for _, v := range allVisited {
		if v == "classify" {
			hasClassify = true
		}
		if v == "investigate" {
			hasInvestigate = true
		}
	}
	if !hasClassify || !hasInvestigate {
		t.Errorf("both nodes should be visited: classify=%v investigate=%v (herald=%v seeker=%v)",
			hasClassify, hasInvestigate, herald.visited, seeker.visited)
	}
}

func TestRun_WithTeam_InputPropagated(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)

	w := &stubWalker{
		identity: roster.AgentIdentity{Name: "Solo"},
		state:    circuit.NewWalkerState("solo-1"),
	}

	err := Run(context.Background(), path, map[string]any{"hello": "world"},
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithCollective([]circuit.Walker{w}, &AffinitySelector{}),
	)
	if err != nil {
		t.Fatalf("Run with team + input: %v", err)
	}

	got, ok := w.State().Context["input"]
	if !ok {
		t.Fatal("expected input in walker context")
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("input type = %T, want map[string]any", got)
	}
	if m["hello"] != "world" {
		t.Errorf("input = %v, want {hello: world}", m)
	}
}

func TestValidate_MissingTransformer(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	err := Validate(path, WithTransformers(TransformerRegistry{}))
	if err == nil {
		t.Fatal("expected error for missing transformer when registry is provided but empty")
	}
}

func TestValidate_NoRegistries_StructuralOnly(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	err := Validate(path)
	if err != nil {
		t.Fatalf("structural validation without registries should pass: %v", err)
	}
}

func TestRun_WithCheckpointer_SavesAfterEachNode(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	cpDir := t.TempDir()
	cp, _ := NewJSONCheckpointer(cpDir)

	trace := &TraceCollector{}
	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithCheckpointer(cp),
		WithRunObserver(trace),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	ids, _ := cp.List()
	if len(ids) != 0 {
		t.Errorf("checkpoint should be removed after successful walk, got %d", len(ids))
	}
}

func TestRun_WithCheckpointer_ResumeFromCheckpoint(t *testing.T) {
	threeNodeYAML := `
circuit: test-resume
nodes:
  - name: a
    approach: fire
    instrument: transformer
    action: echo
  - name: b
    approach: water
    instrument: transformer
    action: echo
  - name: c
    approach: fire
    instrument: transformer
    action: echo
edges:
  - id: E1
    from: a
    to: b
    when: "true"
  - id: E2
    from: b
    to: c
    when: "true"
  - id: E3
    from: c
    to: _done
    when: "true"
start: a
done: _done
`
	path := writeTempCircuit(t, threeNodeYAML)
	cpDir := t.TempDir()
	cp, _ := NewJSONCheckpointer(cpDir)

	saved := circuit.NewWalkerState("run")
	saved.CurrentNode = "b"
	saved.Status = "running"
	saved.RecordStep("a", "E1", "E1", "2026-01-01T00:00:00Z")
	cp.Save(saved)

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithCheckpointer(cp),
		WithResume("run"),
	)
	if err != nil {
		t.Fatalf("Run with resume: %v", err)
	}

	ids, _ := cp.List()
	if len(ids) != 0 {
		t.Errorf("checkpoint should be removed after successful walk, got %d", len(ids))
	}
}

func TestRun_Interrupt_PausesWalk(t *testing.T) {
	yaml := `
circuit: test-interrupt
nodes:
  - name: a
    approach: fire
    instrument: transformer
    action: echo
  - name: b
    approach: water
    instrument: transformer
    action: interrupt-here
  - name: c
    approach: fire
    instrument: transformer
    action: echo
edges:
  - id: E1
    from: a
    to: b
    when: "true"
  - id: E2
    from: b
    to: c
    when: "true"
  - id: E3
    from: c
    to: _done
    when: "true"
start: a
done: _done
`
	path := writeTempCircuit(t, yaml)
	cpDir := t.TempDir()
	cp, _ := NewJSONCheckpointer(cpDir)
	trace := &TraceCollector{}

	interruptTrans := TransformerFunc("interrupt-here", func(_ context.Context, _ *TransformerContext) (any, error) {
		return nil, Interrupt{Reason: "need approval"}
	})

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{
			"echo":           &echoTransformer{},
			"interrupt-here": interruptTrans,
		}),
		WithCheckpointer(cp),
		WithRunObserver(trace),
	)
	if err != nil {
		t.Fatalf("Run with interrupt should not return error, got: %v", err)
	}

	ids, _ := cp.List()
	if len(ids) != 1 {
		t.Fatalf("expected 1 checkpoint (walk interrupted), got %d", len(ids))
	}

	interrupted := trace.EventsOfType(circuit.EventWalkInterrupted)
	if len(interrupted) != 1 {
		t.Errorf("expected 1 interrupt event, got %d", len(interrupted))
	}
	if len(interrupted) > 0 && interrupted[0].Node != "b" {
		t.Errorf("interrupt node = %q, want b", interrupted[0].Node)
	}
}

func TestRun_ResumeWithInput_AfterInterrupt(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)
	cpDir := t.TempDir()
	cp, _ := NewJSONCheckpointer(cpDir)

	saved := circuit.NewWalkerState("resumable")
	saved.CurrentNode = "start"
	saved.Status = "interrupted"
	cp.Save(saved)

	trace := &TraceCollector{}
	w := &stubWalker{
		identity: roster.AgentIdentity{Name: "tester"},
		state:    circuit.NewWalkerState("resumable"),
	}

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithCheckpointer(cp),
		WithResumeInput("resumable", map[string]any{"approved": true}),
		WithWalker(w),
		WithRunObserver(trace),
	)
	if err != nil {
		t.Fatalf("Run with resume input: %v", err)
	}

	ri := w.State().Context["resume_input"]
	if ri == nil {
		t.Fatal("resume_input should be in walker context")
	}
	m, ok := ri.(map[string]any)
	if !ok {
		t.Fatalf("resume_input type = %T, want map[string]any", ri)
	}
	if m["approved"] != true {
		t.Errorf("resume_input = %v, want {approved: true}", m)
	}

	resumed := trace.EventsOfType(circuit.EventWalkResumed)
	if len(resumed) != 1 {
		t.Errorf("expected 1 resume event, got %d", len(resumed))
	}
}

func TestIsInterrupt(t *testing.T) {
	i := Interrupt{Reason: "test"}
	if !IsInterrupt(i) {
		t.Error("IsInterrupt should return true for Interrupt")
	}
	if i.Error() != "interrupt: test" {
		t.Errorf("Error() = %q, want 'interrupt: test'", i.Error())
	}
	if IsInterrupt(nil) {
		t.Error("IsInterrupt(nil) should be false")
	}
	if IsInterrupt(os.ErrNotExist) {
		t.Error("IsInterrupt should return false for non-Interrupt error")
	}
}

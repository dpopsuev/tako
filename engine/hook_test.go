package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestHookRegistry_RegisterAndGet(t *testing.T) {
	reg := HookRegistry{}
	called := false
	h := NewHookFunc("test-hook", func(_ context.Context, _ string, _ circuit.Artifact) error {
		called = true
		return nil
	})
	reg.Register(h)

	got, err := reg.Get("test-hook")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "test-hook" {
		t.Errorf("Name() = %q", got.Name())
	}

	err = got.Run(context.Background(), "node", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !called {
		t.Error("hook was not called")
	}
}

func TestHookRegistry_NotFound(t *testing.T) {
	reg := HookRegistry{}
	_, err := reg.Get("missing")
	if err == nil {
		t.Fatal("expected error for missing hook")
	}
}

func TestHookRegistry_Nil(t *testing.T) {
	var reg HookRegistry
	_, err := reg.Get("any")
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestHookRegistry_DuplicatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate")
		}
	}()
	reg := HookRegistry{}
	h := NewHookFunc("dup", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil })
	reg.Register(h)
	reg.Register(h)
}

func TestHookingWalker_FiresHooks(t *testing.T) {
	var hookCalls []string
	hooks := HookRegistry{}
	hooks.Register(NewHookFunc("h1", func(_ context.Context, nodeName string, _ circuit.Artifact) error {
		hookCalls = append(hookCalls, "h1:"+nodeName)
		return nil
	}))
	hooks.Register(NewHookFunc("h2", func(_ context.Context, nodeName string, _ circuit.Artifact) error {
		hookCalls = append(hookCalls, "h2:"+nodeName)
		return nil
	}))

	trans := &echoTransformer{}
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo", After: []string{"h1", "h2"}},
			{Name: "b", Approach: "analytical", Instrument: "transformer", Action: "echo", After: []string{"h1"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", When: "true"},
			{ID: "E2", From: "b", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": trans},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	expected := []string{"h1:a", "h2:a", "h1:b"}
	if len(hookCalls) != len(expected) {
		t.Fatalf("hook calls = %v, want %v", hookCalls, expected)
	}
	for i, exp := range expected {
		if hookCalls[i] != exp {
			t.Errorf("hookCalls[%d] = %q, want %q", i, hookCalls[i], exp)
		}
	}
}

func TestHookingWalker_MissingHookContinues(t *testing.T) {
	hooks := HookRegistry{}
	trans := &echoTransformer{}
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo", After: []string{"nonexistent"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": trans},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk should succeed even with missing hook: %v", err)
	}
}

func TestFileWriteHook_WritesArtifact(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "recall.json")

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{
				Name:       "recall",
				Approach:   "methodical",
				Instrument: "transformer",
				Action:     "go-template",
				Prompt:     "test data",
				After:      []string{"file-write"},
				Config:     &circuit.NodeConfig{OutputPath: outPath},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "recall", To: "_done", When: "true"},
		},
		Start: "recall",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "recall")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result != "test data" {
		t.Errorf("file content = %v, want %q", result, "test data")
	}
}

func TestFileWriteHook_TemplatedPath(t *testing.T) {
	dir := t.TempDir()
	pathTmpl := filepath.Join(dir, "{{ .NodeName }}.json")

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{
				Name:       "triage",
				Approach:   "rapid",
				Instrument: "transformer",
				Action:     "go-template",
				Prompt:     "triage output",
				After:      []string{"file-write"},
				Config:     &circuit.NodeConfig{OutputPath: pathTmpl},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "triage", To: "_done", When: "true"},
		},
		Start: "triage",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "triage")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	expectedPath := filepath.Join(dir, "triage.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected file at %s", expectedPath)
	}
}

func TestFileWriteHook_MissingOutputPath(t *testing.T) {
	hook := &FileWriteHook{
		NodeConfigs: map[string]*circuit.NodeConfig{
			"node": {},
		},
	}

	art := &transformerArtifact{typeName: "test", raw: "data"}
	err := hook.Run(context.Background(), "node", art)
	if err == nil {
		t.Fatal("expected error for missing output_path")
	}
}

func TestFileWriteHook_AutoRegisteredByRunner(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "auto.json")

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{
				Name:       "a",
				Approach:   "rapid",
				Instrument: "transformer",
				Action:     "passthrough",
				After:      []string{"file-write"},
				Config:     &circuit.NodeConfig{OutputPath: outPath},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("runner should auto-register file-write hook: %v", err)
	}

	if runner.Hooks == nil {
		t.Fatal("hooks should not be nil")
	}
	_, err = runner.Hooks.Get("file-write")
	if err != nil {
		t.Fatalf("file-write hook should be registered: %v", err)
	}
}

func TestBeforeHooks_FireBeforeNodeProcessing(t *testing.T) {
	var order []string
	hooks := HookRegistry{}
	hooks.Register(NewHookFunc("before.inject", func(_ context.Context, nodeName string, art circuit.Artifact) error {
		if art != nil {
			t.Error("before-hook should receive nil artifact")
		}
		order = append(order, "before:"+nodeName)
		return nil
	}))
	hooks.Register(NewHookFunc("after.store", func(_ context.Context, nodeName string, art circuit.Artifact) error {
		if art == nil {
			t.Error("after-hook should receive non-nil artifact")
		}
		order = append(order, "after:"+nodeName)
		return nil
	}))

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo", Before: []string{"before.inject"}, After: []string{"after.store"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": &echoTransformer{}},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	expected := []string{"before:a", "after:a"}
	if len(order) != len(expected) {
		t.Fatalf("hook calls = %v, want %v", order, expected)
	}
	for i, exp := range expected {
		if order[i] != exp {
			t.Errorf("order[%d] = %q, want %q", i, order[i], exp)
		}
	}
}

func TestBeforeHooks_InjectIntoWalkerContext(t *testing.T) {
	hooks := HookRegistry{}

	var capturedState *circuit.WalkerState
	hooks.Register(NewHookFunc("inject.data", func(ctx context.Context, _ string, _ circuit.Artifact) error {
		ws := WalkerStateFromContext(ctx)
		if ws == nil {
			return fmt.Errorf("WalkerStateFromContext returned nil")
		}
		ws.Context["greeting"] = "hello from before-hook"
		return nil
	}))

	captureTrans := TransformerFunc("capture", func(_ context.Context, tc *TransformerContext) (any, error) {
		capturedState = tc.WalkerState
		return "ok", nil
	})

	walker := circuit.NewProcessWalker("test")

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "capture", Before: []string{"inject.data"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"capture": captureTrans},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), walker, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if capturedState == nil {
		t.Fatal("circuit.WalkerState not captured")
	}
	if capturedState.Context["greeting"] != "hello from before-hook" {
		t.Errorf("greeting = %v, want 'hello from before-hook'", capturedState.Context["greeting"])
	}
}

func TestBeforeHooks_OnlyOnDeclaredNodes(t *testing.T) {
	var beforeCalls []string
	hooks := HookRegistry{}
	hooks.Register(NewHookFunc("inject.data", func(_ context.Context, nodeName string, _ circuit.Artifact) error {
		beforeCalls = append(beforeCalls, nodeName)
		return nil
	}))

	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo", Before: []string{"inject.data"}},
			{Name: "b", Approach: "analytical", Instrument: "transformer", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", When: "true"},
			{ID: "E2", From: "b", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": &echoTransformer{}},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(beforeCalls) != 1 || beforeCalls[0] != "a" {
		t.Errorf("before-hook calls = %v, want [a]", beforeCalls)
	}
}

func TestBeforeHooks_MissingHookContinues(t *testing.T) {
	hooks := HookRegistry{}
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo", Before: []string{"nonexistent"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": &echoTransformer{}},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk should succeed even with missing before-hook: %v", err)
	}
}

func TestNodeDef_BeforeParsedFromYAML(t *testing.T) {
	yaml := `
circuit: test-before
nodes:
  - name: recall
    element: earth
    transformer: echo
    before: [inject.envelope, inject.history]
    after: [store.recall]
edges:
  - id: E1
    from: recall
    to: _done
    when: "true"
start: recall
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	nd := def.Nodes[0]
	if len(nd.Before) != 2 {
		t.Fatalf("Before = %v, want 2 entries", nd.Before)
	}
	if nd.Before[0] != "inject.envelope" || nd.Before[1] != "inject.history" {
		t.Errorf("Before = %v", nd.Before)
	}
	if len(nd.After) != 1 || nd.After[0] != "store.recall" {
		t.Errorf("After = %v", nd.After)
	}
}

func TestHookingWalker_NoHooksNoWrap(t *testing.T) {
	trans := &echoTransformer{}
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes:       []circuit.NodeDef{{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo"}},
		Edges:       []circuit.EdgeDef{{ID: "E1", From: "a", To: "_done", When: "true"}},
		Start:       "a",
		Done:        "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"echo": trans},
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

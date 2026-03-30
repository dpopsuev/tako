package engine

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// captureDiagLogs runs fn with a log handler that captures output, returns log text.
func captureDiagLogs(fn func()) string {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	old := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(old)
	fn()
	return buf.String()
}

func TestDiag_D1_UnreferencedHook(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough", After: []string{"hook-a"}},
		},
		Edges: []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		Hooks: HookRegistry{
			"hook-a": NewHookFunc("hook-a", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
			"hook-b": NewHookFunc("hook-b", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
		},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if !diagContains(logs, "hook-b") || !diagContains(logs, "D1") {
		t.Errorf("expected D1 warning for unreferenced hook-b, got:\n%s", logs)
	}
	if diagContains(logs, `hook=hook-a`) {
		t.Errorf("hook-a is referenced, should not be warned about:\n%s", logs)
	}
}

func TestDiag_D2_MissingHookRef(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough", Before: []string{"hook-exists", "hook-missing"}},
		},
		Edges: []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		Hooks: HookRegistry{
			"hook-exists": NewHookFunc("hook-exists", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
		},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if !diagContains(logs, "hook-missing") || !diagContains(logs, "D2") {
		t.Errorf("expected D2 warning for missing hook-missing, got:\n%s", logs)
	}
	if !diagContains(logs, "missing_count=1") {
		t.Errorf("expected missing_count=1, got:\n%s", logs)
	}
}

func TestDiag_D4_PartialHookWiring(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough",
				Before: []string{"hook-a", "hook-b", "hook-c"}},
		},
		Edges: []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		Hooks: HookRegistry{
			"hook-a": NewHookFunc("hook-a", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
		},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if !diagContains(logs, "missing_count=2") {
		t.Errorf("expected missing_count=2, got:\n%s", logs)
	}
	if !diagContains(logs, "declared_count=3") {
		t.Errorf("expected declared_count=3, got:\n%s", logs)
	}
}

func TestDiag_AllHooksReferenced_NoWarnings(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough",
				Before: []string{"hook-a"}, After: []string{"hook-b"}},
		},
		Edges: []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		Hooks: HookRegistry{
			"hook-a": NewHookFunc("hook-a", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
			"hook-b": NewHookFunc("hook-b", func(_ context.Context, _ string, _ circuit.Artifact) error { return nil }),
		},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if diagContains(logs, "WARN") {
		t.Errorf("expected no warnings when all hooks referenced, got:\n%s", logs)
	}
}

func TestDiag_NoHooksRegistered_NoWarnings(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough"},
		},
		Edges: []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if diagContains(logs, "WARN") {
		t.Errorf("expected no warnings with empty hook registry, got:\n%s", logs)
	}
}

func TestDiag_D3_CircuitMediatorFallback(t *testing.T) {
	// A handler_type:circuit node that resolves via mediator (no local circuit)
	// should emit a D3 warning.
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough"},
			{Name: "sub", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-sub", From: "a", To: "sub"},
			{ID: "sub-done", From: "sub", To: "done"},
		},
	}
	reg := &GraphRegistries{
		Transformers:     TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		MediatorEndpoint: "http://localhost:9000/mcp",
		// Note: no Circuits map — gnd will fall back to mediator
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if !diagContains(logs, "D3") {
		t.Errorf("expected D3 warning for mediator fallback, got:\n%s", logs)
	}
	if !diagContains(logs, "gnd") {
		t.Errorf("expected handler name 'gnd' in warning, got:\n%s", logs)
	}
}

func TestDiag_D3_NoWarningForLocalCircuit(t *testing.T) {
	// When the circuit resolves locally, no D3 warning should be emitted.
	innerDef := &circuit.CircuitDef{
		Circuit: "gnd", Start: "x", Done: "done",
		Nodes: []circuit.NodeDef{
			{Name: "x", HandlerType: "transformer", Handler: "passthrough"},
		},
		Edges: []circuit.EdgeDef{{ID: "x-done", From: "x", To: "done"}},
	}
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "a", Done: "done",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough"},
			{Name: "sub", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-sub", From: "a", To: "sub"},
			{ID: "sub-done", From: "sub", To: "done"},
		},
	}
	reg := &GraphRegistries{
		Transformers:     TransformerRegistry{"passthrough": TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })},
		MediatorEndpoint: "http://localhost:9000/mcp",
		Circuits:         map[string]*circuit.CircuitDef{"gnd": innerDef},
	}

	logs := captureDiagLogs(func() {
		_, err := BuildGraph(def, reg)
		if err != nil {
			t.Fatalf("BuildGraph: %v", err)
		}
	})

	if diagContains(logs, "D3") {
		t.Errorf("expected no D3 warning when circuit resolves locally, got:\n%s", logs)
	}
}

func diagContains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}

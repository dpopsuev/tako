package testkit

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/testkit/builders"
)

func TestRunCircuit_LinearWalk(t *testing.T) {
	def := builders.NewCircuitDef("test").
		AddNodeWithInstrument("step-a", "transformer", "echo").
		AddNodeWithInstrument("step-b", "transformer", "echo").
		AddEdge("step-a", "step-b", "").
		AddEdge("step-b", "_done", "").
		Start("step-a").
		Done("_done").
		Build()

	transformers := engine.InstrumentRegistry{
		"echo": engine.InstrumentFunc("echo", func(_ context.Context, tc *engine.InstrumentContext) (any, error) {
			return map[string]any{"node": tc.NodeName}, nil
		}),
	}

	result := RunCircuit(context.Background(), def, transformers)

	if result.Error != nil {
		t.Fatalf("RunCircuit: %v", result.Error)
	}
	if len(result.Path) != 2 {
		t.Fatalf("path = %v, want [step-a, step-b]", result.Path)
	}
	if result.Path[0] != "step-a" || result.Path[1] != "step-b" {
		t.Errorf("path = %v, want [step-a, step-b]", result.Path)
	}
	if !result.Visited("step-a") {
		t.Error("step-a not visited")
	}
	if !result.Visited("step-b") {
		t.Error("step-b not visited")
	}
}

func TestRunCircuit_BranchingWalk(t *testing.T) {
	def := builders.NewCircuitDef("branch").
		AddNodeWithInstrument("check", "transformer", "echo").
		AddNodeWithInstrument("pass", "transformer", "echo").
		AddEdge("check", "pass", "").
		AddEdge("pass", "_done", "").
		Start("check").
		Done("_done").
		Build()

	transformers := engine.InstrumentRegistry{
		"echo": engine.InstrumentFunc("echo", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
			return "ok", nil
		}),
	}

	result := RunCircuit(context.Background(), def, transformers)

	if result.Error != nil {
		t.Fatalf("RunCircuit: %v", result.Error)
	}
	if len(result.Enters()) != 2 {
		t.Errorf("enters = %d, want 2", len(result.Enters()))
	}
	if len(result.Exits()) != 2 {
		t.Errorf("exits = %d, want 2", len(result.Exits()))
	}
}

func TestRunCircuit_MissingTransformer(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "bad",
		Start:   "a",
		Done:    "_done",
		Nodes:   []circuit.NodeDef{{Name: "a", Instrument: "transformer", Action: "nonexistent"}},
		Edges:   []circuit.EdgeDef{{ID: "e1", From: "a", To: "_done"}},
	}

	result := RunCircuit(context.Background(), def, engine.InstrumentRegistry{})

	if result.Error == nil {
		t.Fatal("expected error for missing transformer")
	}
}

func TestRunCircuit_WithInput(t *testing.T) {
	def := builders.NewCircuitDef("input-test").
		AddNodeWithInstrument("step", "transformer", "echo").
		AddEdge("step", "_done", "").
		Start("step").
		Done("_done").
		Build()

	transformers := engine.InstrumentRegistry{
		"echo": engine.InstrumentFunc("echo", func(_ context.Context, tc *engine.InstrumentContext) (any, error) {
			return tc.WalkerState.Context["input"], nil
		}),
	}

	result := RunCircuit(context.Background(), def, transformers, WithInput("hello"))

	if result.Error != nil {
		t.Fatalf("RunCircuit: %v", result.Error)
	}
	if !result.Visited("step") {
		t.Error("step not visited")
	}
}

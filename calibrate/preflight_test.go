package calibrate

import (
	"context"
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
)

func TestPreflight_ValidCircuit(t *testing.T) {
	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: testCircuitDef(),
	})
	if err != nil {
		t.Fatalf("expected valid circuit to pass preflight, got: %v", err)
	}
}

func TestPreflight_MissingTransformer(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit:     "broken-circuit",
		HandlerType: "transformer",
		Start:       "start",
		Done:        "done",
		Nodes: []framework.NodeDef{
			{Name: "start", HandlerType: "transformer", Handler: "nonexistent-transformer"},
		},
		Edges: []framework.EdgeDef{
			{ID: "start-done", From: "start", To: "done"},
		},
	}

	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for missing transformer, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestPreflight_InvalidStartNode(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "bad-start",
		Start:   "nonexistent",
		Done:    "done",
		Nodes: []framework.NodeDef{
			{Name: "start", HandlerType: "transformer", Handler: "passthrough"},
		},
		Edges: []framework.EdgeDef{
			{ID: "start-done", From: "start", To: "done"},
		},
	}

	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for invalid start node, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestPreflight_NilCircuitDef(t *testing.T) {
	err := Preflight(context.Background(), HarnessConfig{})
	if err == nil {
		t.Fatal("expected error for nil CircuitDef, got nil")
	}
}

func TestPreflight_CompletesQuickly(t *testing.T) {
	start := time.Now()
	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: testCircuitDef(),
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("preflight took %v, expected < 1s", elapsed)
	}
	t.Logf("preflight completed in %v", elapsed)
}

func TestPreflight_MultiNodeCircuit(t *testing.T) {
	// Circuit with multiple nodes; preflight should still pass because
	// it only validates graph construction and enters the start node.
	def := &framework.CircuitDef{
		Circuit:     "multi-node",
		HandlerType: "transformer",
		Start:       "a",
		Done:        "done",
		Nodes: []framework.NodeDef{
			{Name: "a", HandlerType: "transformer", Handler: "passthrough"},
			{Name: "b", HandlerType: "transformer", Handler: "passthrough"},
		},
		Edges: []framework.EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
			{ID: "b-done", From: "b", To: "done"},
		},
	}

	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: def,
	})
	if err != nil {
		t.Fatalf("expected multi-node circuit to pass preflight, got: %v", err)
	}
}

func TestPreflight_BrokenEdgeExpression(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit:     "broken-edge",
		HandlerType: "transformer",
		Start:       "start",
		Done:        "done",
		Nodes: []framework.NodeDef{
			{Name: "start", HandlerType: "transformer", Handler: "passthrough"},
		},
		Edges: []framework.EdgeDef{
			{ID: "start-done", From: "start", To: "done", When: "invalid_func(!!!"},
		},
	}

	err := Preflight(context.Background(), HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for broken edge expression, got nil")
	}
	t.Logf("got expected error: %v", err)
}

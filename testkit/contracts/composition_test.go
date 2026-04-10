package contracts_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// TestComposition_OverlayWithCircuitHandler verifies that an overlay
// circuit adding an instrument:circuit node resolves correctly when
// MediatorEndpoint is set. This is the E2E contract that would have
// caught the gather-code silent skip bug.
func TestComposition_OverlayWithCircuitHandler(t *testing.T) {
	// Base circuit: a → done
	baseDef := &circuit.CircuitDef{
		Circuit: "base",
		Start:   "a",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "a", To: "done"},
		},
	}

	// Overlay adds: a → sub → done (sub is instrument:circuit)
	overlayDef := &circuit.CircuitDef{
		Circuit: "overlay",
		Start:   "a",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a", Instrument: "transformer", Action: "passthrough"},
			{Name: "sub", Instrument: "circuit", Action: "inner"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-sub", From: "a", To: "sub"},
			{ID: "sub-done", From: "sub", To: "done"},
		},
	}

	// Inner circuit definition (resolved locally).
	innerDef := &circuit.CircuitDef{
		Circuit: "inner",
		Start:   "x",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "x", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "x-done", From: "x", To: "done"},
		},
	}

	passthrough := engine.TransformerFunc("passthrough", func(_ context.Context, tc *engine.TransformerContext) (any, error) {
		return tc.Input, nil
	})

	t.Run("local circuit resolves", func(t *testing.T) {
		reg := &engine.GraphRegistries{
			Transformers: engine.TransformerRegistry{"passthrough": passthrough},
			Circuits:     map[string]*circuit.CircuitDef{"inner": innerDef},
		}
		_, err := engine.BuildGraph(overlayDef, reg)
		if err != nil {
			t.Fatalf("BuildGraph with local circuit should succeed: %v", err)
		}
	})

	t.Run("mediator fallback resolves", func(t *testing.T) {
		reg := &engine.GraphRegistries{
			Transformers:     engine.TransformerRegistry{"passthrough": passthrough},
			MediatorEndpoint: "http://localhost:9999/mcp",
			// No Circuits — should fall back to MCPCircuitTransformer
		}
		_, err := engine.BuildGraph(overlayDef, reg)
		if err != nil {
			t.Fatalf("BuildGraph with mediator endpoint should succeed: %v", err)
		}
	})

	t.Run("no circuit and no mediator fails", func(t *testing.T) {
		reg := &engine.GraphRegistries{
			Transformers: engine.TransformerRegistry{"passthrough": passthrough},
			// No Circuits, no MediatorEndpoint
		}
		_, err := engine.BuildGraph(overlayDef, reg)
		if err == nil {
			t.Fatal("BuildGraph should fail when circuit node has no local circuit and no mediator")
		}
	})

	t.Run("base circuit without overlay succeeds", func(t *testing.T) {
		reg := &engine.GraphRegistries{
			Transformers: engine.TransformerRegistry{"passthrough": passthrough},
		}
		_, err := engine.BuildGraph(baseDef, reg)
		if err != nil {
			t.Fatalf("BuildGraph for base circuit should succeed: %v", err)
		}
	})

	t.Run("walk visits all overlay nodes with local circuit", func(t *testing.T) {
		reg := &engine.GraphRegistries{
			Transformers: engine.TransformerRegistry{"passthrough": passthrough},
			Circuits:     map[string]*circuit.CircuitDef{"inner": innerDef},
		}
		cases := []engine.BatchCase{{ID: "C1"}}
		results := engine.BatchWalk(context.Background(), engine.BatchWalkConfig{
			Def:      overlayDef,
			Shared:   reg,
			Cases:    cases,
			Parallel: 1,
		})
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Error != nil {
			t.Fatalf("walk error: %v", results[0].Error)
		}
		// Both "a" and "sub" should have been visited.
		visited := make(map[string]bool)
		for _, n := range results[0].Path {
			visited[n] = true
		}
		if !visited["a"] {
			t.Error("node 'a' not visited")
		}
		if !visited["sub"] {
			t.Error("node 'sub' not visited — overlay node silently skipped")
		}
	})
}

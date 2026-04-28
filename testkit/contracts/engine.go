package contracts

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// RunBatchWalkContract verifies that a BatchWalk implementation produces
// correct results for a simple circuit. The factory must return a
// function with the same signature as engine.BatchWalk.
func RunBatchWalkContract(t *testing.T, batchWalk func(context.Context, engine.BatchWalkConfig) []engine.BatchWalkResult, graphFactory func(*circuit.CircuitDef, *engine.GraphRegistries) (engine.Graph, error)) {
	t.Helper()

	t.Run("SingleCase_ProducesOneResult", func(t *testing.T) {
		def := &circuit.CircuitDef{
			Circuit: "contract-test",
			Start:   "a",
			Done:    "_done",
			Nodes: []circuit.NodeDef{
				{Name: "a", Instrument: "transformer", Action: "echo"},
			},
			Edges: []circuit.EdgeDef{
				{ID: "e1", From: "a", To: "_done"},
			},
		}

		transformers := engine.InstrumentRegistry{
			"echo": engine.InstrumentFunc("echo", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
				return "ok", nil
			}),
		}

		results := batchWalk(context.Background(), engine.BatchWalkConfig{
			Def:    def,
			Shared: &engine.GraphRegistries{Instruments: transformers},
			Cases:  []engine.BatchCase{{ID: "test-1"}},
		})

		if len(results) != 1 {
			t.Fatalf("results = %d, want 1", len(results))
		}
		if results[0].Error != nil {
			t.Errorf("walk error: %v", results[0].Error)
		}
	})

	t.Run("MultipleCases_ProducesMatchingResults", func(t *testing.T) {
		def := &circuit.CircuitDef{
			Circuit: "multi",
			Start:   "a",
			Done:    "_done",
			Nodes: []circuit.NodeDef{
				{Name: "a", Instrument: "transformer", Action: "echo"},
			},
			Edges: []circuit.EdgeDef{
				{ID: "e1", From: "a", To: "_done"},
			},
		}

		transformers := engine.InstrumentRegistry{
			"echo": engine.InstrumentFunc("echo", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
				return "ok", nil
			}),
		}

		cases := []engine.BatchCase{
			{ID: "case-1"},
			{ID: "case-2"},
			{ID: "case-3"},
		}

		results := batchWalk(context.Background(), engine.BatchWalkConfig{
			Def:    def,
			Shared: &engine.GraphRegistries{Instruments: transformers},
			Cases:  cases,
		})

		if len(results) != 3 {
			t.Fatalf("results = %d, want 3", len(results))
		}
	})
}

// RunTuneContract verifies that TuneAll correctly validates instruments.
func RunTuneContract(t *testing.T, tuneAll func(context.Context, engine.ManifestRegistry, string) error) {
	t.Helper()

	t.Run("EmptyRegistry_NoError", func(t *testing.T) {
		err := tuneAll(context.Background(), engine.ManifestRegistry{}, "")
		if err != nil {
			t.Errorf("TuneAll with empty registry: %v", err)
		}
	})

	t.Run("NilRegistry_NoError", func(t *testing.T) {
		err := tuneAll(context.Background(), nil, "")
		if err != nil {
			t.Errorf("TuneAll with nil registry: %v", err)
		}
	})
}

package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestWalk_Finally_RunsOnSuccess(t *testing.T) {
	pt := TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })
	var finallyCalled bool
	ft := TransformerFunc("cleanup", func(_ context.Context, _ *TransformerContext) (any, error) {
		finallyCalled = true
		return map[string]any{"cleaned": true}, nil
	})

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Start:       "step-a",
		Done:        "done",
		Finally:     "teardown",
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Handler: "passthrough"},
			{Name: "teardown", Handler: "cleanup"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "step-a", To: "done"},
		},
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": pt, "cleanup": ft},
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		t.Fatal(err)
	}

	walker := circuit.NewProcessWalker("test-walker")
	if err := runner.Walk(context.Background(), walker, "step-a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if !finallyCalled {
		t.Error("finally node was not called on successful walk")
	}
	if _, ok := walker.State().Outputs["teardown"]; !ok {
		t.Error("finally artifact not in outputs")
	}
}

func TestWalk_Finally_RunsOnError(t *testing.T) {
	failing := TransformerFunc("fail", func(_ context.Context, _ *TransformerContext) (any, error) {
		return nil, circuit.ErrNodeNotFound // any error
	})
	var finallyCalled bool
	ft := TransformerFunc("cleanup", func(_ context.Context, tc *TransformerContext) (any, error) {
		finallyCalled = true
		// Verify the walk error is accessible in context.
		if tc.WalkerState.Context["_walk_error"] == nil {
			t.Error("_walk_error not set in context during finally")
		}
		return map[string]any{"cleaned": true}, nil
	})

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Start:       "step-a",
		Done:        "done",
		Finally:     "teardown",
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Handler: "fail"},
			{Name: "teardown", Handler: "cleanup"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "step-a", To: "done"},
		},
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"fail": failing, "cleanup": ft},
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		t.Fatal(err)
	}

	walker := circuit.NewProcessWalker("test-walker")
	walkErr := runner.Walk(context.Background(), walker, "step-a")

	if walkErr == nil {
		t.Fatal("expected walk error")
	}
	if !finallyCalled {
		t.Error("finally node was not called on failed walk")
	}
}

func TestWalk_Finally_RunsOnCanceledContext(t *testing.T) {
	pt := TransformerFunc("slow", func(ctx context.Context, _ *TransformerContext) (any, error) {
		return nil, ctx.Err() // will be canceled
	})
	var finallyCalled bool
	ft := TransformerFunc("cleanup", func(_ context.Context, _ *TransformerContext) (any, error) {
		finallyCalled = true
		return nil, nil
	})

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Start:       "step-a",
		Done:        "done",
		Finally:     "teardown",
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Handler: "slow"},
			{Name: "teardown", Handler: "cleanup"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "step-a", To: "done"},
		},
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"slow": pt, "cleanup": ft},
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	walker := circuit.NewProcessWalker("test-walker")
	_ = runner.Walk(ctx, walker, "step-a")

	if !finallyCalled {
		t.Error("finally node was not called on canceled context")
	}
}

func TestWalk_NoFinally_SkipsCleanup(t *testing.T) {
	pt := TransformerFunc("passthrough", func(_ context.Context, tc *TransformerContext) (any, error) { return tc.Input, nil })

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Start:       "step-a",
		Done:        "done",
		// No Finally set
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Handler: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "step-a", To: "done"},
		},
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": pt},
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		t.Fatal(err)
	}

	walker := circuit.NewProcessWalker("test-walker")
	if err := runner.Walk(context.Background(), walker, "step-a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	// No crash, no error — just verifies the no-finally path is clean.
}

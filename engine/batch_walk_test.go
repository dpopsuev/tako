package engine

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func batchTestDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "batch-test",
		Start:       "step-a",
		Done:        "_done",
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Instrument: "transformer", Action: "echo"},
			{Name: "step-b", Instrument: "transformer", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "step-a", To: "step-b", When: "true"},
			{ID: "E2", From: "step-b", To: "_done", When: "true"},
		},
	}
}

func batchTestRegistries() *GraphRegistries {
	return &GraphRegistries{
		Instruments: InstrumentRegistry{
			"echo": InstrumentFunc("echo", func(_ context.Context, tc *InstrumentContext) (any, error) {
				return map[string]any{"node": tc.NodeName, "input": tc.WalkerState.Context["case_input"]}, nil
			}),
		},
	}
}

func TestBatchWalk_Serial(t *testing.T) {
	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:    batchTestDef(),
		Shared: batchTestRegistries(),
		Cases: []BatchCase{
			{ID: "c1", Context: map[string]any{"case_input": "alpha"}},
			{ID: "c2", Context: map[string]any{"case_input": "beta"}},
			{ID: "c3", Context: map[string]any{"case_input": "gamma"}},
		},
		Parallel: 1,
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("case %d (%s): %v", i, r.CaseID, r.Error)
		}
		if len(r.Path) != 2 {
			t.Errorf("case %s: path len = %d, want 2", r.CaseID, len(r.Path))
		}
		if r.Path[0] != "step-a" || r.Path[1] != "step-b" {
			t.Errorf("case %s: path = %v", r.CaseID, r.Path)
		}
		if r.StepArtifacts["step-a"] == nil || r.StepArtifacts["step-b"] == nil {
			t.Errorf("case %s: missing step artifacts", r.CaseID)
		}
		if r.State == nil || r.State.Status != "done" {
			t.Errorf("case %s: state.Status = %q, want done", r.CaseID, r.State.Status)
		}
	}

	if results[0].CaseID != "c1" || results[1].CaseID != "c2" || results[2].CaseID != "c3" {
		t.Errorf("results not in input order: %s, %s, %s", results[0].CaseID, results[1].CaseID, results[2].CaseID)
	}
}

func TestBatchWalk_Parallel(t *testing.T) {
	var concurrency atomic.Int32
	var maxConcurrency atomic.Int32

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"echo": InstrumentFunc("echo", func(_ context.Context, tc *InstrumentContext) (any, error) {
				cur := concurrency.Add(1)
				for {
					old := maxConcurrency.Load()
					if cur <= old || maxConcurrency.CompareAndSwap(old, cur) {
						break
					}
				}
				defer concurrency.Add(-1)
				return map[string]any{"node": tc.NodeName}, nil
			}),
		},
	}

	cases := make([]BatchCase, 10)
	for i := range cases {
		cases[i] = BatchCase{ID: "c" + string(rune('0'+i))}
	}

	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:      batchTestDef(),
		Shared:   reg,
		Cases:    cases,
		Parallel: 4,
	})

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("case %s: %v", r.CaseID, r.Error)
		}
	}
}

func TestBatchWalk_PerCaseComponents(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "hook-test",
		Start:       "step-a",
		Done:        "_done",
		Nodes: []circuit.NodeDef{
			{Name: "step-a", Instrument: "transformer", Action: "echo", After: []string{"track"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "step-a", To: "_done", When: "true"},
		},
	}

	sharedReg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"echo": InstrumentFunc("echo", func(_ context.Context, tc *InstrumentContext) (any, error) {
				return map[string]any{"node": tc.NodeName}, nil
			}),
		},
	}

	hookCalled := make(map[string]bool)
	hookComponent := &Component{
		Namespace: "test",
		Name:      "case-hook",
		Hooks: HookRegistry{
			"track": NewHookFunc("track", func(_ context.Context, _ string, _ circuit.Artifact) error {
				hookCalled["with-hook"] = true
				return nil
			}),
		},
	}

	noopHookComponent := &Component{
		Namespace: "test",
		Name:      "noop-hook",
		Hooks: HookRegistry{
			"track": NewHookFunc("track", func(_ context.Context, _ string, _ circuit.Artifact) error {
				return nil
			}),
		},
	}

	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:    def,
		Shared: sharedReg,
		Cases: []BatchCase{
			{ID: "with-hook", Components: []*Component{hookComponent}},
			{ID: "no-hook", Components: []*Component{noopHookComponent}},
		},
		Parallel: 1,
	})

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("case %s: %v", r.CaseID, r.Error)
		}
	}

	if !hookCalled["with-hook"] {
		t.Error("hook was not called for 'with-hook' case")
	}
}

func TestBatchWalk_Empty(t *testing.T) {
	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:    batchTestDef(),
		Shared: batchTestRegistries(),
		Cases:  nil,
	})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestBatchWalk_CaseError(t *testing.T) {
	badDef := &circuit.CircuitDef{
		Circuit:     "bad",
		Start:       "missing-node",
		Done:        "_done",
		Nodes:       []circuit.NodeDef{{Name: "step-a", Instrument: "transformer", Action: "echo"}},
		Edges:       []circuit.EdgeDef{{ID: "E1", From: "step-a", To: "_done", When: "true"}},
	}

	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:    badDef,
		Shared: batchTestRegistries(),
		Cases:  []BatchCase{{ID: "c1"}},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error for bad circuit")
	}
}

package reactivity

import (
	"testing"
)

func TestCatalyst_Ready_NoPreconditions(t *testing.T) {
	c := &Catalyst{
		Desired: map[string]any{"done": true},
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"x": 1}},
			{Need: "B", Current: map[string]any{}, Desired: map[string]any{"y": 2}},
		},
	}
	ready := c.Ready(map[string]any{})
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready steps, got %d", len(ready))
	}
}

func TestCatalyst_Ready_BlockedByPrecondition(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"code": "fixed"}},
			{Need: "B", Current: map[string]any{"code": "fixed"}, Desired: map[string]any{"tests": "pass"}},
		},
	}
	actual := map[string]any{"code": "broken"}
	ready := c.Ready(actual)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready (A), got %d", len(ready))
	}
	if ready[0].Need != "A" {
		t.Errorf("expected A to be ready, got %s", ready[0].Need)
	}
}

func TestCatalyst_Ready_AfterStateChange(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"code": "fixed"}},
			{Need: "B", Current: map[string]any{"code": "fixed"}, Desired: map[string]any{"tests": "pass"}},
		},
	}
	actual := map[string]any{"code": "fixed"}
	ready := c.Ready(actual)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready (both A and B), got %d", len(ready))
	}
}

func TestCatalyst_DependsOn(t *testing.T) {
	a := &Catalyst{Current: map[string]any{}, Desired: map[string]any{"code": "fixed"}}
	b := &Catalyst{Current: map[string]any{"code": "fixed"}, Desired: map[string]any{"tests": "pass"}}
	c := &Catalyst{Current: map[string]any{}, Desired: map[string]any{"docs": "updated"}}

	if !b.DependsOn(a) {
		t.Error("B should depend on A (both touch 'code')")
	}
	if b.DependsOn(c) {
		t.Error("B should not depend on C (no shared dimensions)")
	}
	if a.DependsOn(b) {
		t.Error("A should not depend on B")
	}
}

func TestCatalyst_Completed(t *testing.T) {
	c := &Catalyst{Desired: map[string]any{"x": 1, "y": 2}}
	if c.Completed(map[string]any{"x": 1}) {
		t.Error("should not be completed with partial state")
	}
	if !c.Completed(map[string]any{"x": 1, "y": 2, "z": 3}) {
		t.Error("should be completed when all desired dimensions match")
	}
}

func TestPlan_ParallelBatching(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "fix code", Current: map[string]any{}, Desired: map[string]any{"code": "fixed"}},
			{Need: "update docs", Current: map[string]any{}, Desired: map[string]any{"docs": "updated"}},
			{Need: "run tests", Current: map[string]any{"code": "fixed"}, Desired: map[string]any{"tests": "pass"}},
			{Need: "create PR", Current: map[string]any{"tests": "pass", "docs": "updated"}, Desired: map[string]any{"pr": "ready"}},
		},
	}

	plan := Plan(c)

	if len(plan.Batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(plan.Batches))
	}

	// Batch 0: fix code + update docs (parallel, no deps)
	if len(plan.Batches[0]) != 2 {
		t.Errorf("batch 0 should have 2 parallel steps, got %d", len(plan.Batches[0]))
	}

	// Batch 1: run tests (depends on code fixed)
	if len(plan.Batches[1]) != 1 {
		t.Errorf("batch 1 should have 1 step, got %d", len(plan.Batches[1]))
	}
	if plan.Batches[1][0].Need != "run tests" {
		t.Errorf("batch 1 should be 'run tests', got %s", plan.Batches[1][0].Need)
	}

	// Batch 2: create PR (depends on tests + docs)
	if len(plan.Batches[2]) != 1 {
		t.Errorf("batch 2 should have 1 step, got %d", len(plan.Batches[2]))
	}
	if plan.Batches[2][0].Need != "create PR" {
		t.Errorf("batch 2 should be 'create PR', got %s", plan.Batches[2][0].Need)
	}
}

func TestPlan_LinearChain(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"a": true}},
			{Need: "B", Current: map[string]any{"a": true}, Desired: map[string]any{"b": true}},
			{Need: "C", Current: map[string]any{"b": true}, Desired: map[string]any{"c": true}},
		},
	}
	plan := Plan(c)
	if len(plan.Batches) != 3 {
		t.Fatalf("expected 3 sequential batches, got %d", len(plan.Batches))
	}
	for i, batch := range plan.Batches {
		if len(batch) != 1 {
			t.Errorf("batch %d should have 1 step, got %d", i, len(batch))
		}
	}
}

func TestPlan_AllParallel(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"a": true}},
			{Need: "B", Current: map[string]any{}, Desired: map[string]any{"b": true}},
			{Need: "C", Current: map[string]any{}, Desired: map[string]any{"c": true}},
		},
	}
	plan := Plan(c)
	if len(plan.Batches) != 1 {
		t.Fatalf("expected 1 batch (all parallel), got %d", len(plan.Batches))
	}
	if len(plan.Batches[0]) != 3 {
		t.Errorf("batch should have 3 steps, got %d", len(plan.Batches[0]))
	}
}

func TestPlanFromState_DynamicResolution(t *testing.T) {
	c := &Catalyst{
		Steps: []*Catalyst{
			{Need: "A", Current: map[string]any{}, Desired: map[string]any{"x": 1}},
			{Need: "B", Current: map[string]any{"x": 1}, Desired: map[string]any{"y": 2}},
		},
	}

	completed := make(map[*Catalyst]bool)

	// Initially only A is ready
	ready := PlanFromState(c, map[string]any{}, completed)
	if len(ready) != 1 || ready[0].Need != "A" {
		t.Fatalf("expected [A], got %v", ready)
	}

	// Mark A complete, update state
	completed[c.Steps[0]] = true
	ready = PlanFromState(c, map[string]any{"x": 1}, completed)
	if len(ready) != 1 || ready[0].Need != "B" {
		t.Fatalf("expected [B], got %v", ready)
	}

	// Mark B complete
	completed[c.Steps[1]] = true
	ready = PlanFromState(c, map[string]any{"x": 1, "y": 2}, completed)
	if len(ready) != 0 {
		t.Fatalf("expected no ready steps, got %d", len(ready))
	}
}

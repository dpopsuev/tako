package mcp

import (
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// --- TSK-516: computeStepLatency ---

func TestComputeStepLatency_Basic(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeExit, Node: "step-a", Elapsed: 100 * time.Millisecond},
		{Type: circuit.EventNodeExit, Node: "step-a", Elapsed: 200 * time.Millisecond},
		{Type: circuit.EventNodeExit, Node: "step-a", Elapsed: 300 * time.Millisecond},
		{Type: circuit.EventNodeExit, Node: "step-b", Elapsed: 50 * time.Millisecond},
	}

	latency := computeStepLatency(events)

	if len(latency) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(latency))
	}

	a := latency["step-a"]
	if a.Count != 3 {
		t.Errorf("step-a count: want 3, got %d", a.Count)
	}
	if a.Min != 100*time.Millisecond {
		t.Errorf("step-a min: want 100ms, got %v", a.Min)
	}
	if a.Max != 300*time.Millisecond {
		t.Errorf("step-a max: want 300ms, got %v", a.Max)
	}
	if a.Mean != 200*time.Millisecond {
		t.Errorf("step-a mean: want 200ms, got %v", a.Mean)
	}
	if a.P50 != 200*time.Millisecond {
		t.Errorf("step-a p50: want 200ms, got %v", a.P50)
	}

	b := latency["step-b"]
	if b.Count != 1 {
		t.Errorf("step-b count: want 1, got %d", b.Count)
	}
	if b.P50 != 50*time.Millisecond {
		t.Errorf("step-b p50: want 50ms, got %v", b.P50)
	}
}

func TestComputeStepLatency_IgnoresNonExit(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "step-a", Elapsed: 100 * time.Millisecond},
		{Type: circuit.EventEdgeEvaluate, Node: "step-a", Elapsed: 50 * time.Millisecond},
	}

	latency := computeStepLatency(events)
	if len(latency) != 0 {
		t.Errorf("expected 0 nodes (non-exit events), got %d", len(latency))
	}
}

func TestComputeStepLatency_IgnoresZeroElapsed(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeExit, Node: "step-a", Elapsed: 0},
	}

	latency := computeStepLatency(events)
	if len(latency) != 0 {
		t.Errorf("expected 0 nodes (zero elapsed), got %d", len(latency))
	}
}

func TestComputeStepLatency_Empty(t *testing.T) {
	latency := computeStepLatency(nil)
	if len(latency) != 0 {
		t.Errorf("expected 0 nodes for nil events, got %d", len(latency))
	}
}

// --- Percentile ---

func TestPercentile_SingleValue(t *testing.T) {
	sorted := []time.Duration{42 * time.Millisecond}
	if got := percentile(sorted, 50); got != 42*time.Millisecond {
		t.Errorf("p50 of single value: want 42ms, got %v", got)
	}
	if got := percentile(sorted, 99); got != 42*time.Millisecond {
		t.Errorf("p99 of single value: want 42ms, got %v", got)
	}
}

func TestPercentile_Empty(t *testing.T) {
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("p50 of empty: want 0, got %v", got)
	}
}

func TestPercentile_TwoValues(t *testing.T) {
	sorted := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}
	p50 := percentile(sorted, 50)
	// p50 of [100, 200] with linear interpolation = 150ms
	if p50 != 150*time.Millisecond {
		t.Errorf("p50: want 150ms, got %v", p50)
	}
}

// --- TSK-517: computeErrorRates ---

func TestComputeErrorRates_NoErrors(t *testing.T) {
	results := []engine.BatchWalkResult{
		{CaseID: "C01", Path: []string{"step-a", "step-b"}},
		{CaseID: "C02", Path: []string{"step-a", "step-b"}},
	}

	rates := computeErrorRates(results)

	if len(rates) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(rates))
	}
	for node, rate := range rates {
		if rate.ErrorCount != 0 {
			t.Errorf("%s: want 0 errors, got %d", node, rate.ErrorCount)
		}
		if rate.ErrorRate != 0 {
			t.Errorf("%s: want 0.0 error rate, got %f", node, rate.ErrorRate)
		}
	}
}

func TestComputeErrorRates_WithErrors(t *testing.T) {
	results := []engine.BatchWalkResult{
		{CaseID: "C01", Path: []string{"step-a", "step-b"}, Error: nil},
		{CaseID: "C02", Path: []string{"step-a", "step-b"}, Error: errTestWalk},
		{CaseID: "C03", Path: []string{"step-a"}, Error: errTestWalk},
	}

	rates := computeErrorRates(results)

	// step-a was visited by all 3 cases, errored for C03 (last node in path).
	a := rates["step-a"]
	if a.TotalCases != 3 {
		t.Errorf("step-a total: want 3, got %d", a.TotalCases)
	}
	if a.ErrorCount != 1 {
		t.Errorf("step-a errors: want 1, got %d", a.ErrorCount)
	}

	// step-b was visited by 2 cases, errored for C02 (last node in path).
	b := rates["step-b"]
	if b.TotalCases != 2 {
		t.Errorf("step-b total: want 2, got %d", b.TotalCases)
	}
	if b.ErrorCount != 1 {
		t.Errorf("step-b errors: want 1, got %d", b.ErrorCount)
	}
	wantRate := 0.5
	if b.ErrorRate != wantRate {
		t.Errorf("step-b error rate: want %f, got %f", wantRate, b.ErrorRate)
	}
}

func TestComputeErrorRates_Empty(t *testing.T) {
	rates := computeErrorRates(nil)
	if len(rates) != 0 {
		t.Errorf("expected 0 nodes for nil results, got %d", len(rates))
	}
}

func TestComputeErrorRates_ErrorWithEmptyPath(t *testing.T) {
	results := []engine.BatchWalkResult{
		{CaseID: "C01", Path: nil, Error: errTestWalk},
	}

	rates := computeErrorRates(results)
	if len(rates) != 0 {
		t.Errorf("expected 0 nodes for empty path error, got %d", len(rates))
	}
}

var errTestWalk = errors.New("test walk error")

package calibrate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// failTransformer errors when the walker's case ID starts with "fail-".
type failTransformer struct{}

func (failTransformer) Name() string { return "fail-on-id" }
func (failTransformer) Transform(_ context.Context, tc *engine.InstrumentContext) (any, error) {
	if tc.WalkerState != nil && strings.HasPrefix(tc.WalkerState.ID, "fail-") {
		return nil, fmt.Errorf("injected circuit error for case %s", tc.WalkerState.ID)
	}
	return "ok", nil
}

func errorRateCircuitDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "error-rate-test",
		Start:   "start",
		Done:    "done",
		Nodes:   []circuit.NodeDef{{Name: "start", Instrument: "transformer", Action: "fail-on-id"}},
		Edges:   []circuit.EdgeDef{{ID: "start-done", From: "start", To: "done"}},
	}
}

func errorRateShared() *engine.GraphRegistries {
	return &engine.GraphRegistries{
		Instruments: engine.InstrumentRegistry{
			"fail-on-id": failTransformer{},
		},
	}
}

func TestErrorRate_NoErrors_NoGate(t *testing.T) {
	loader := &mockLoader{cases: []engine.BatchCase{
		{ID: "ok-1", Context: map[string]any{}},
		{ID: "ok-2", Context: map[string]any{}},
		{ID: "ok-3", Context: map[string]any{}},
	}}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 1.0, "M2": 1.0},
		details: map[string]string{},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:       loader,
		Collector:    collector,
		CircuitDef:   errorRateCircuitDef(),
		ScoreCard:    testScoreCard(),
		Shared:       errorRateShared(),
		MaxErrorRate: 0.10,
		Scenario:     "no-errors",
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

func TestErrorRate_AtThreshold_Passes(t *testing.T) {
	// 1/10 = 10% error rate, threshold = 10% → passes (not strictly greater)
	cases := make([]engine.BatchCase, 10)
	cases[0] = engine.BatchCase{ID: "fail-1", Context: map[string]any{}}
	for i := 1; i < 10; i++ {
		cases[i] = engine.BatchCase{ID: fmt.Sprintf("ok-%d", i), Context: map[string]any{}}
	}

	loader := &mockLoader{cases: cases}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.9, "M2": 0.9},
		details: map[string]string{},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:       loader,
		Collector:    collector,
		CircuitDef:   errorRateCircuitDef(),
		ScoreCard:    testScoreCard(),
		Shared:       errorRateShared(),
		MaxErrorRate: 0.10,
		Scenario:     "at-threshold",
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

func TestErrorRate_ExceedsThreshold_Fails(t *testing.T) {
	// 3/10 = 30% error rate, threshold = 10% → fails
	cases := make([]engine.BatchCase, 10)
	cases[0] = engine.BatchCase{ID: "fail-1", Context: map[string]any{}}
	cases[1] = engine.BatchCase{ID: "fail-2", Context: map[string]any{}}
	cases[2] = engine.BatchCase{ID: "fail-3", Context: map[string]any{}}
	for i := 3; i < 10; i++ {
		cases[i] = engine.BatchCase{ID: fmt.Sprintf("ok-%d", i), Context: map[string]any{}}
	}

	loader := &mockLoader{cases: cases}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.7, "M2": 0.7},
		details: map[string]string{},
	}

	_, err := Run(context.Background(), &HarnessConfig{
		Loader:       loader,
		Collector:    collector,
		CircuitDef:   errorRateCircuitDef(),
		ScoreCard:    testScoreCard(),
		Shared:       errorRateShared(),
		MaxErrorRate: 0.10,
		Scenario:     "exceeds-threshold",
	})
	if err == nil {
		t.Fatal("expected error for exceeding error rate threshold")
	}
	if !strings.Contains(err.Error(), "circuit error rate") {
		t.Errorf("error message should mention 'circuit error rate', got: %v", err)
	}
	if !strings.Contains(err.Error(), "30%") {
		t.Errorf("error message should mention '30%%', got: %v", err)
	}
	if !strings.Contains(err.Error(), "3/10") {
		t.Errorf("error message should mention '3/10', got: %v", err)
	}
	if !strings.Contains(err.Error(), "exceeds threshold 10%") {
		t.Errorf("error message should mention 'exceeds threshold 10%%', got: %v", err)
	}
	if !strings.Contains(err.Error(), "first error:") {
		t.Errorf("error message should mention 'first error:', got: %v", err)
	}
}

func TestErrorRate_DefaultZero_NoGate(t *testing.T) {
	// MaxErrorRate=0 (default) → no gate, even with 50% errors
	cases := make([]engine.BatchCase, 10)
	for i := 0; i < 5; i++ {
		cases[i] = engine.BatchCase{ID: fmt.Sprintf("fail-%d", i+1), Context: map[string]any{}}
	}
	for i := 5; i < 10; i++ {
		cases[i] = engine.BatchCase{ID: fmt.Sprintf("ok-%d", i), Context: map[string]any{}}
	}

	loader := &mockLoader{cases: cases}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.5, "M2": 0.5},
		details: map[string]string{},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:     loader,
		Collector:  collector,
		CircuitDef: errorRateCircuitDef(),
		ScoreCard:  testScoreCard(),
		Shared:     errorRateShared(),
		// MaxErrorRate is 0 (zero value) — no gate
		Scenario: "default-no-gate",
	})
	if err != nil {
		t.Fatalf("Run() unexpected error with MaxErrorRate=0: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

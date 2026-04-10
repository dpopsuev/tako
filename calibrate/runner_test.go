package calibrate

import (
	"context"
	"fmt"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// --- Mock implementations ---

type mockLoader struct {
	cases []engine.BatchCase
	err   error
	calls int
}

func (m *mockLoader) Load(_ context.Context) ([]engine.BatchCase, error) {
	m.calls++
	return m.cases, m.err
}

type mockCollector struct {
	values  map[string]float64
	details map[string]string
	err     error
	calls   int
}

func (m *mockCollector) Collect(_ context.Context, _ []engine.BatchWalkResult) (values map[string]float64, details map[string]string, err error) {
	m.calls++
	return m.values, m.details, m.err
}

func testScoreCard() *ScoreCard {
	return &ScoreCard{
		Name: "test",
		MetricDefs: []MetricDef{
			{ID: "M1", Name: "accuracy", Tier: TierOutcome, Direction: HigherIsBetter, Threshold: 0.80, Weight: 1.0},
			{ID: "M2", Name: "recall", Tier: TierDetection, Direction: HigherIsBetter, Threshold: 0.70, Weight: 1.0},
		},
	}
}

func testCircuitDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "test-circuit",
		Start:   "start",
		Done:    "done",
		Nodes:   []circuit.NodeDef{{Name: "start", Instrument: "transformer", Action: "passthrough"}},
		Edges:   []circuit.EdgeDef{{ID: "start-done", From: "start", To: "done"}},
	}
}

// --- Tests ---

func TestRun_MissingLoader(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{})
	if err == nil {
		t.Fatal("expected error for missing Loader")
	}
}

func TestRun_MissingCollector(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{
		Loader: &mockLoader{},
	})
	if err == nil {
		t.Fatal("expected error for missing Collector")
	}
}

func TestRun_MissingCircuitDef(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{
		Loader:    &mockLoader{},
		Collector: &mockCollector{},
	})
	if err == nil {
		t.Fatal("expected error for missing CircuitDef")
	}
}

func TestRun_MissingScoreCard(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{
		Loader:     &mockLoader{},
		Collector:  &mockCollector{},
		CircuitDef: testCircuitDef(),
	})
	if err == nil {
		t.Fatal("expected error for missing ScoreCard")
	}
}

func TestRun_LoaderError(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{
		Loader:     &mockLoader{err: fmt.Errorf("load failed")},
		Collector:  &mockCollector{},
		CircuitDef: testCircuitDef(),
		ScoreCard:  testScoreCard(),
		Scenario:   "test",
	})
	if err == nil {
		t.Fatal("expected error from loader")
	}
}

func TestRun_CollectorError(t *testing.T) {
	_, err := Run(context.Background(), &HarnessConfig{
		Loader: &mockLoader{cases: []engine.BatchCase{
			{ID: "C1", Context: map[string]any{}},
		}},
		Collector:  &mockCollector{err: fmt.Errorf("collect failed")},
		CircuitDef: testCircuitDef(),
		ScoreCard:  testScoreCard(),
		Scenario:   "test",
	})
	if err == nil {
		t.Fatal("expected error from collector")
	}
}

func TestRun_SingleRun(t *testing.T) {
	loader := &mockLoader{cases: []engine.BatchCase{
		{ID: "C1", Context: map[string]any{}},
	}}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.90, "M2": 0.85},
		details: map[string]string{"M1": "9/10", "M2": "17/20"},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:      loader,
		Collector:   collector,
		CircuitDef:  testCircuitDef(),
		ScoreCard:   testScoreCard(),
		Scenario:    "ptp-mock",
		Transformer: "stub",
		Runs:        1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Scenario != "ptp-mock" {
		t.Errorf("Scenario = %q, want ptp-mock", report.Scenario)
	}
	if report.Transformer != "stub" {
		t.Errorf("Transformer = %q, want stub", report.Transformer)
	}
	if report.Runs != 1 {
		t.Errorf("Runs = %d, want 1", report.Runs)
	}

	passed, total := report.Metrics.PassCount()
	if passed != 2 || total != 2 {
		t.Errorf("PassCount = %d/%d, want 2/2", passed, total)
	}
	if loader.calls != 1 {
		t.Errorf("loader.calls = %d, want 1", loader.calls)
	}
	if collector.calls != 1 {
		t.Errorf("collector.calls = %d, want 1", collector.calls)
	}
}

func TestRun_MultiRun(t *testing.T) {
	loader := &mockLoader{cases: []engine.BatchCase{
		{ID: "C1", Context: map[string]any{}},
	}}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.85, "M2": 0.75},
		details: map[string]string{},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:      loader,
		Collector:   collector,
		CircuitDef:  testCircuitDef(),
		ScoreCard:   testScoreCard(),
		Scenario:    "multi",
		Transformer: "stub",
		Runs:        3,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Runs != 3 {
		t.Errorf("Runs = %d, want 3", report.Runs)
	}
	if len(report.RunMetrics) != 3 {
		t.Errorf("RunMetrics = %d, want 3", len(report.RunMetrics))
	}
	if loader.calls != 3 {
		t.Errorf("loader.calls = %d, want 3", loader.calls)
	}
	if collector.calls != 3 {
		t.Errorf("collector.calls = %d, want 3", collector.calls)
	}
}

func TestNewHarnessConfig(t *testing.T) {
	loader := &mockLoader{}
	collector := &mockCollector{}
	def := testCircuitDef()
	sc := testScoreCard()
	shared := &engine.GraphRegistries{}
	contract := &CalibrationContract{}
	stubs := PortStubs{"stub1": "data1"}

	cfg := NewHarnessConfig(
		WithLoader(loader),
		WithCollector(collector),
		WithCircuitDef(def),
		WithScoreCard(sc),
		WithShared(shared),
		WithContract(contract),
		WithResolution(ResolutionUnit, &ResolutionPlan{Name: "plan1"}),
		WithPortStubs(stubs),
		WithWalkerContext(map[string]any{"key": "val"}),
		WithMaxErrorRate(0.15),
		WithScenario("test-scenario"),
		WithTransformerName("my-transformer"),
		WithRuns(5),
		WithParallel(3),
	)

	if cfg.Loader != loader {
		t.Error("Loader not set")
	}
	if cfg.Collector != collector {
		t.Error("Collector not set")
	}
	if cfg.CircuitDef != def {
		t.Error("CircuitDef not set")
	}
	if cfg.ScoreCard != sc {
		t.Error("ScoreCard not set")
	}
	if cfg.Shared != shared {
		t.Error("Shared not set")
	}
	if cfg.Contract != contract {
		t.Error("Contract not set")
	}
	if cfg.Resolution != ResolutionUnit {
		t.Errorf("Resolution = %q, want %q", cfg.Resolution, ResolutionUnit)
	}
	if cfg.Plan == nil || cfg.Plan.Name != "plan1" {
		t.Error("Plan not set correctly")
	}
	if cfg.PortStubs["stub1"] != "data1" {
		t.Error("PortStubs not set")
	}
	if cfg.WalkerContext["key"] != "val" {
		t.Error("WalkerContext not set")
	}
	if cfg.MaxErrorRate != 0.15 {
		t.Errorf("MaxErrorRate = %f, want 0.15", cfg.MaxErrorRate)
	}
	if cfg.Scenario != "test-scenario" {
		t.Errorf("Scenario = %q, want test-scenario", cfg.Scenario)
	}
	if cfg.Transformer != "my-transformer" {
		t.Errorf("Transformer = %q, want my-transformer", cfg.Transformer)
	}
	if cfg.Runs != 5 {
		t.Errorf("Runs = %d, want 5", cfg.Runs)
	}
	if cfg.Parallel != 3 {
		t.Errorf("Parallel = %d, want 3", cfg.Parallel)
	}
}

func TestNewHarnessConfig_Empty(t *testing.T) {
	cfg := NewHarnessConfig()
	if cfg == nil {
		t.Fatal("NewHarnessConfig returned nil")
	}
	if cfg.Runs != 0 {
		t.Errorf("Runs = %d, want 0", cfg.Runs)
	}
	if cfg.Scenario != "" {
		t.Errorf("Scenario = %q, want empty", cfg.Scenario)
	}
}

func TestNewHarnessConfig_CompatibleWithRun(t *testing.T) {
	loader := &mockLoader{cases: []engine.BatchCase{
		{ID: "C1", Context: map[string]any{}},
	}}
	collector := &mockCollector{
		values:  map[string]float64{"M1": 0.90, "M2": 0.85},
		details: map[string]string{"M1": "9/10", "M2": "17/20"},
	}

	cfg := NewHarnessConfig(
		WithLoader(loader),
		WithCollector(collector),
		WithCircuitDef(testCircuitDef()),
		WithScoreCard(testScoreCard()),
		WithScenario("options-compat"),
		WithTransformerName("stub"),
		WithRuns(1),
	)

	report, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run with NewHarnessConfig: %v", err)
	}
	if report.Scenario != "options-compat" {
		t.Errorf("Scenario = %q, want options-compat", report.Scenario)
	}
	passed, total := report.Metrics.PassCount()
	if passed != 2 || total != 2 {
		t.Errorf("PassCount = %d/%d, want 2/2", passed, total)
	}
}

func TestRun_DefaultRuns(t *testing.T) {
	loader := &mockLoader{cases: []engine.BatchCase{
		{ID: "C1", Context: map[string]any{}},
	}}
	collector := &mockCollector{
		values: map[string]float64{"M1": 1.0, "M2": 1.0},
	}

	report, err := Run(context.Background(), &HarnessConfig{
		Loader:     loader,
		Collector:  collector,
		CircuitDef: testCircuitDef(),
		ScoreCard:  testScoreCard(),
		Scenario:   "default-run",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Runs != 1 {
		t.Errorf("Runs = %d, want 1 (default)", report.Runs)
	}
}

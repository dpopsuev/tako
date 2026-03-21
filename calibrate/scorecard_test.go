package calibrate_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/calibrate"
)

// --- MetricDef ---

func TestMetricDef_Evaluate_HigherIsBetter(t *testing.T) {
	d := calibrate.MetricDef{Threshold: 0.80, Direction: calibrate.HigherIsBetter}
	if !d.Evaluate(0.80) {
		t.Error("0.80 >= 0.80 should pass")
	}
	if !d.Evaluate(0.90) {
		t.Error("0.90 >= 0.80 should pass")
	}
	if d.Evaluate(0.79) {
		t.Error("0.79 < 0.80 should fail")
	}
}

func TestMetricDef_Evaluate_LowerIsBetter(t *testing.T) {
	d := calibrate.MetricDef{Threshold: 200000, Direction: calibrate.LowerIsBetter}
	if !d.Evaluate(100000) {
		t.Error("100000 <= 200000 should pass")
	}
	if !d.Evaluate(200000) {
		t.Error("200000 <= 200000 should pass")
	}
	if d.Evaluate(200001) {
		t.Error("200001 > 200000 should fail")
	}
}

func TestMetricDef_Evaluate_RangeCheck(t *testing.T) {
	d := calibrate.MetricDef{Threshold: 1.0, Direction: calibrate.RangeCheck}
	if !d.Evaluate(0.5) {
		t.Error("0.5 in [0,1] should pass")
	}
	if d.Evaluate(-0.1) {
		t.Error("-0.1 below 0 should fail")
	}
	if d.Evaluate(1.1) {
		t.Error("1.1 above threshold should fail")
	}
}

func TestMetricDef_Evaluate_DefaultDirection(t *testing.T) {
	d := calibrate.MetricDef{Threshold: 0.50}
	if !d.Evaluate(0.50) {
		t.Error("empty direction defaults to higher_is_better")
	}
}

func TestMetricDef_ToMetric(t *testing.T) {
	d := calibrate.MetricDef{
		ID: "M1", Name: "Accuracy",
		Tier: calibrate.TierOutcome, Direction: calibrate.HigherIsBetter,
		Threshold: 0.80, DryCapped: true,
	}
	m := d.ToMetric(0.85, "test detail")
	if m.ID != "M1" || m.Name != "Accuracy" {
		t.Error("ID/Name mismatch")
	}
	if !m.Pass {
		t.Error("0.85 >= 0.80 should pass")
	}
	if m.Detail != "test detail" {
		t.Error("detail mismatch")
	}
	if !m.DryCapped {
		t.Error("DryCapped should propagate")
	}
	if m.Tier != calibrate.TierOutcome {
		t.Error("Tier should propagate")
	}
}

// --- CostModel ---

func TestCostModel_ROI(t *testing.T) {
	cm := calibrate.CostModel{
		CasesPerBatch:                    20,
		CostPerBatchUSD:                  1.00,
		LaborSavedPerBatchPersonDays:     100,
		PersonDayCostUSD:                 500,
	}
	roi := cm.ROI()
	// savings = 100 * 500 = 50000, cost = 1, ROI = (50000-1)/1 = 49999
	if math.Abs(roi-49999) > 1e-6 {
		t.Errorf("ROI: want 49999, got %.2f", roi)
	}
}

func TestCostModel_ROI_ZeroCost(t *testing.T) {
	cm := calibrate.CostModel{CostPerBatchUSD: 0}
	if cm.ROI() != 0 {
		t.Error("zero cost should return 0")
	}
}

// --- ScoreCard ---

func testScoreCard() calibrate.ScoreCard {
	return calibrate.NewScoreCardBuilder("test").
		WithDescription("Test scorecard").
		WithMetrics(
			calibrate.MetricDef{ID: "M1", Name: "Accuracy", Tier: calibrate.TierOutcome, Direction: calibrate.HigherIsBetter, Threshold: 0.80, Weight: 0.50},
			calibrate.MetricDef{ID: "M2", Name: "Coverage", Tier: calibrate.TierOutcome, Direction: calibrate.HigherIsBetter, Threshold: 0.70, Weight: 0.30},
			calibrate.MetricDef{ID: "M18", Name: "TokenUsage", Tier: calibrate.TierEfficiency, Direction: calibrate.LowerIsBetter, Threshold: 200000, Weight: 0},
		).
		WithAggregate(calibrate.AggregateConfig{
			ID: "M19", Name: "Overall", Formula: "weighted_average",
			Threshold: 0.70, Include: []string{"M1", "M2"},
		}).
		Build()
}

func TestScoreCard_FindDef(t *testing.T) {
	sc := testScoreCard()
	if d := sc.FindDef("M1"); d == nil || d.Name != "Accuracy" {
		t.Error("FindDef(M1) failed")
	}
	if d := sc.FindDef("MISSING"); d != nil {
		t.Error("FindDef(MISSING) should return nil")
	}
}

func TestScoreCard_Evaluate(t *testing.T) {
	sc := testScoreCard()
	vals := map[string]float64{"M1": 0.85, "M2": 0.60, "M18": 150000}
	details := map[string]string{"M1": "good"}
	ms := sc.Evaluate(vals, details)
	all := ms.AllMetrics()
	if len(all) != 3 {
		t.Fatalf("want 3 metrics, got %d", len(all))
	}
	byID := ms.ByID()
	if !byID["M1"].Pass {
		t.Error("M1: 0.85 >= 0.80 should pass")
	}
	if byID["M2"].Pass {
		t.Error("M2: 0.60 < 0.70 should fail")
	}
	if !byID["M18"].Pass {
		t.Error("M18: 150000 <= 200000 should pass (lower_is_better)")
	}
	if byID["M1"].Detail != "good" {
		t.Error("detail should propagate")
	}
}

func TestScoreCard_Evaluate_MissingValues(t *testing.T) {
	sc := testScoreCard()
	ms := sc.Evaluate(map[string]float64{"M1": 0.90}, nil)
	if len(ms.AllMetrics()) != 1 {
		t.Error("should only produce metrics for provided values")
	}
}

func TestScoreCard_ComputeAggregate(t *testing.T) {
	sc := testScoreCard()
	vals := map[string]float64{"M1": 0.90, "M2": 0.80, "M18": 100000}
	ms := sc.Evaluate(vals, nil)
	agg, err := sc.ComputeAggregate(ms)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// weighted_average: (0.90*0.50 + 0.80*0.30) / (0.50+0.30) = (0.45+0.24)/0.80 = 0.8625
	if math.Abs(agg.Value-0.8625) > 1e-4 {
		t.Errorf("agg value: want ~0.8625, got %.4f", agg.Value)
	}
	if !agg.Pass {
		t.Error("agg should pass (0.86 >= 0.70)")
	}
}

func TestScoreCard_ComputeAggregate_NoConfig(t *testing.T) {
	sc := calibrate.ScoreCard{}
	_, err := sc.ComputeAggregate(calibrate.MetricSet{})
	if err == nil {
		t.Error("should error when no aggregate config")
	}
}

func TestScoreCard_Report(t *testing.T) {
	sc := testScoreCard()
	vals := map[string]float64{"M1": 0.85, "M2": 0.80, "M18": 100000}
	r, err := sc.Report("test-scenario", "stub", 1, vals, nil)
	if err != nil {
		t.Fatalf("Report error: %v", err)
	}
	if r.Scenario != "test-scenario" {
		t.Error("scenario mismatch")
	}
	if len(r.Metrics.Metrics) != 4 {
		t.Errorf("want 4 metrics (3 + agg), got %d", len(r.Metrics.Metrics))
	}
}

// --- ValidateScorers ---

func TestScoreCard_ValidateScorers_AllExist(t *testing.T) {
	sc := calibrate.NewScoreCardBuilder("test").
		WithMetrics(
			calibrate.MetricDef{ID: "M1", Name: "Accuracy", Scorer: "accuracy", Params: map[string]any{"predicted": "a", "expected": "b"}},
			calibrate.MetricDef{ID: "M2", Name: "Rate", Scorer: "rate", Params: map[string]any{"field": "items"}},
			calibrate.MetricDef{ID: "M3", Name: "NoScorer"},
		).
		Build()

	reg := calibrate.DefaultScorerRegistry()
	if err := sc.ValidateScorers(reg); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestScoreCard_ValidateScorers_MissingScorers(t *testing.T) {
	sc := calibrate.NewScoreCardBuilder("test").
		WithMetrics(
			calibrate.MetricDef{ID: "M1", Name: "Good", Scorer: "accuracy"},
			calibrate.MetricDef{ID: "M2", Name: "Bad1", Scorer: "nonexistent_alpha"},
			calibrate.MetricDef{ID: "M3", Name: "Bad2", Scorer: "nonexistent_beta"},
			calibrate.MetricDef{ID: "M4", Name: "NoScorer"},
		).
		Build()

	reg := calibrate.DefaultScorerRegistry()
	err := sc.ValidateScorers(reg)
	if err == nil {
		t.Fatal("expected error for missing scorers")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "nonexistent_alpha") {
		t.Errorf("error should mention nonexistent_alpha, got: %s", errStr)
	}
	if !strings.Contains(errStr, "nonexistent_beta") {
		t.Errorf("error should mention nonexistent_beta, got: %s", errStr)
	}
}

// --- DefaultMetrics ---

func TestDefaultMetrics_Count(t *testing.T) {
	defs := calibrate.DefaultMetrics()
	if len(defs) != 9 {
		t.Errorf("want 9 default metrics, got %d", len(defs))
	}
}

func TestDefaultMetrics_IDs(t *testing.T) {
	defs := calibrate.DefaultMetrics()
	ids := make(map[string]bool)
	for _, d := range defs {
		if ids[d.ID] {
			t.Errorf("duplicate ID: %s", d.ID)
		}
		ids[d.ID] = true
	}
	want := []string{"token_usage", "token_cost_usd", "latency_seconds", "path_efficiency", "loop_ratio", "confidence_calibration", "run_variance", "evidence_snr", "walker_mismatch"}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("missing default metric: %s", id)
		}
	}
}

func TestDefaultMetrics_AllHaveDirection(t *testing.T) {
	for _, d := range calibrate.DefaultMetrics() {
		if d.Direction == "" {
			t.Errorf("metric %s has empty direction", d.ID)
		}
	}
}

// --- DefaultScoreCard ---

func TestDefaultScoreCard_ContainsUniversalMetrics(t *testing.T) {
	sc := calibrate.DefaultScoreCard().Build()
	if len(sc.MetricDefs) != 9 {
		t.Errorf("DefaultScoreCard should have 9 metrics, got %d", len(sc.MetricDefs))
	}
	if sc.Name != "default" {
		t.Errorf("name: want 'default', got %q", sc.Name)
	}
}

func TestDefaultScoreCard_ExtendWithDomain(t *testing.T) {
	sc := calibrate.DefaultScoreCard().
		WithMetrics(calibrate.MetricDef{ID: "domain_m1", Name: "Domain Metric"}).
		Build()
	if len(sc.MetricDefs) != 10 {
		t.Errorf("want 10 (9+1), got %d", len(sc.MetricDefs))
	}
}

// --- ScoreCardBuilder ---

func TestScoreCardBuilder_FullChain(t *testing.T) {
	sc := calibrate.NewScoreCardBuilder("my-card").
		WithDescription("desc").
		WithCostModel(calibrate.CostModel{CostPerBatchUSD: 1.0}).
		WithMetrics(calibrate.MetricDef{ID: "X"}).
		WithAggregate(calibrate.AggregateConfig{ID: "AGG"}).
		Build()
	if sc.Name != "my-card" {
		t.Error("name")
	}
	if sc.Description != "desc" {
		t.Error("description")
	}
	if sc.CostModel == nil || sc.CostModel.CostPerBatchUSD != 1.0 {
		t.Error("cost model")
	}
	if len(sc.MetricDefs) != 1 {
		t.Error("metrics")
	}
	if sc.Aggregate == nil || sc.Aggregate.ID != "AGG" {
		t.Error("aggregate")
	}
}

// --- LoadScoreCard ---

func TestLoadScoreCard_YAML(t *testing.T) {
	yamlContent := `
scorecard: test-card
description: "Test"
version: 1
cost_model:
  cases_per_batch: 20
  cost_per_batch_usd: 1.0
  labor_saved_per_batch_person_days: 100
  person_day_cost_usd: 500
metrics:
  - id: M1
    name: accuracy
    tier: outcome
    direction: higher_is_better
    threshold: 0.85
    weight: 0.25
  - id: M18
    name: tokens
    tier: efficiency
    direction: lower_is_better
    threshold: 200000
    weight: 0.0
aggregate:
  id: M19
  name: overall
  formula: weighted_average
  threshold: 0.70
  include: [M1]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "scorecard.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	sc, err := calibrate.LoadScoreCard(path)
	if err != nil {
		t.Fatalf("LoadScoreCard: %v", err)
	}
	if sc.Name != "test-card" {
		t.Errorf("name: want 'test-card', got %q", sc.Name)
	}
	if len(sc.MetricDefs) != 2 {
		t.Errorf("metrics: want 2, got %d", len(sc.MetricDefs))
	}
	if sc.CostModel == nil {
		t.Fatal("cost_model missing")
	}
	if sc.CostModel.ROI() < 49000 {
		t.Errorf("ROI too low: %.2f", sc.CostModel.ROI())
	}
	if sc.Aggregate == nil || sc.Aggregate.ID != "M19" {
		t.Error("aggregate missing")
	}
}

func TestLoadScoreCard_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scorecard.json")
	os.WriteFile(path, []byte(`{}`), 0644)
	_, err := calibrate.LoadScoreCard(path)
	if err == nil {
		t.Error("should reject .json format")
	}
}

func TestLoadScoreCard_MissingFile(t *testing.T) {
	_, err := calibrate.LoadScoreCard("/nonexistent/scorecard.yaml")
	if err == nil {
		t.Error("should error on missing file")
	}
}

// --- Three-Layer Pattern Integration ---

func TestThreeLayerPattern(t *testing.T) {
	domainMetric := calibrate.MetricDef{
		ID: "domain_accuracy", Name: "Domain Accuracy",
		Tier: calibrate.TierOutcome, Direction: calibrate.HigherIsBetter,
		Threshold: 0.85, Weight: 0.50,
	}

	sc := calibrate.DefaultScoreCard().
		WithMetrics(domainMetric).
		WithAggregate(calibrate.AggregateConfig{
			ID: "overall", Name: "Overall",
			Formula: "weighted_average", Threshold: 0.70,
			Include: []string{"domain_accuracy", "confidence_calibration"},
		}).
		Build()

	if len(sc.MetricDefs) != 10 {
		t.Fatalf("want 10 (9 default + 1 domain), got %d", len(sc.MetricDefs))
	}

	vals := map[string]float64{
		"token_usage":              150000,
		"token_cost_usd":          2.50,
		"latency_seconds":         120,
		"path_efficiency":         0.75,
		"loop_ratio":              0.10,
		"confidence_calibration":  0.70,
		"run_variance":            0.05,
		"domain_accuracy":         0.90,
	}
	report, err := sc.Report("integration", "stub", 1, vals, nil)
	if err != nil {
		t.Fatalf("Report error: %v", err)
	}

	passed, total := report.Metrics.PassCount()
	if total != 9 {
		t.Errorf("want 9 metrics (8 + agg), got %d", total)
	}
	if passed != total {
		t.Errorf("all should pass, got %d/%d", passed, total)
	}
}

package ergograph

import (
	"math"
	"testing"
)

func TestMetricDef_Evaluate_HigherIsBetter(t *testing.T) {
	d := MetricDef{Threshold: 0.8, Direction: HigherIsBetter}
	if !d.Evaluate(0.9) {
		t.Error("0.9 >= 0.8 should pass")
	}
	if d.Evaluate(0.7) {
		t.Error("0.7 < 0.8 should fail")
	}
}

func TestMetricDef_Evaluate_LowerIsBetter(t *testing.T) {
	d := MetricDef{Threshold: 0.2, Direction: LowerIsBetter}
	if !d.Evaluate(0.1) {
		t.Error("0.1 <= 0.2 should pass")
	}
	if d.Evaluate(0.3) {
		t.Error("0.3 > 0.2 should fail")
	}
}

func TestMetricDef_ToMetric(t *testing.T) {
	d := MetricDef{ID: "acc", Name: "Accuracy", Threshold: 0.8, Direction: HigherIsBetter, Tier: TierOutcome}
	m := d.ToMetric(0.9, "good")
	if m.ID != "acc" {
		t.Errorf("id: %s", m.ID)
	}
	if !m.Pass {
		t.Error("should pass")
	}
	if m.Tier != TierOutcome {
		t.Errorf("tier: %s", m.Tier)
	}
}

func TestMetricSet_PassCount(t *testing.T) {
	ms := MetricSet{Metrics: []Metric{
		{Pass: true}, {Pass: false}, {Pass: true},
	}}
	passed, total := ms.PassCount()
	if passed != 2 || total != 3 {
		t.Errorf("pass count: %d/%d", passed, total)
	}
}

func TestMetricSet_ByTier(t *testing.T) {
	ms := MetricSet{Metrics: []Metric{
		{Tier: TierOutcome}, {Tier: TierEfficiency}, {Tier: TierOutcome},
	}}
	tiers := ms.ByTier()
	if len(tiers[TierOutcome]) != 2 {
		t.Errorf("outcome count: %d", len(tiers[TierOutcome]))
	}
}

func TestScorerRegistry_RegisterAndGet(t *testing.T) {
	reg := make(ScorerRegistry)
	reg.Register("test", func(_, _ any, _ map[string]any) (float64, string, error) {
		return 1.0, "ok", nil
	})
	fn, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	val, _, _ := fn(nil, nil, nil)
	if val != 1.0 {
		t.Errorf("value: %f", val)
	}
}

func TestScorerRegistry_GetMissing(t *testing.T) {
	reg := make(ScorerRegistry)
	_, err := reg.Get("nope")
	if err == nil {
		t.Error("expected error for missing scorer")
	}
}

func TestMean(t *testing.T) {
	if Mean(nil) != 0 {
		t.Error("empty mean should be 0")
	}
	if Mean([]float64{2, 4, 6}) != 4 {
		t.Errorf("mean: %f", Mean([]float64{2, 4, 6}))
	}
}

func TestStddev(t *testing.T) {
	if Stddev(nil) != 0 {
		t.Error("empty stddev should be 0")
	}
	sd := Stddev([]float64{2, 4, 6})
	if math.Abs(sd-2.0) > 0.01 {
		t.Errorf("stddev: %f", sd)
	}
}

func TestAggregateRunMetrics(t *testing.T) {
	runs := []MetricSet{
		{Metrics: []Metric{{ID: "a", Value: 0.6, Threshold: 0.5}}},
		{Metrics: []Metric{{ID: "a", Value: 0.8, Threshold: 0.5}}},
	}
	agg := AggregateRunMetrics(runs, nil)
	if len(agg.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(agg.Metrics))
	}
	if agg.Metrics[0].Value != 0.7 {
		t.Errorf("aggregate value: %f", agg.Metrics[0].Value)
	}
	if !agg.Metrics[0].Pass {
		t.Error("0.7 >= 0.5 should pass")
	}
}

func TestSafeDiv(t *testing.T) {
	if SafeDiv(0, 0) != 1.0 {
		t.Error("0/0 should be 1.0")
	}
	if SafeDiv(3, 4) != 0.75 {
		t.Errorf("3/4: %f", SafeDiv(3, 4))
	}
}

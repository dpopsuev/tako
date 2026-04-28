package builders_test

import (
	"testing"

	"github.com/dpopsuev/tako/calibrate"
	"github.com/dpopsuev/tako/testkit/builders"
)

func TestScoreCardBuilder_Basic(t *testing.T) {
	sc := builders.NewScoreCard().
		AddMetric("M1", "accuracy", 0.8).
		Build()

	if sc.Name != "test-scorecard" {
		t.Errorf("Name = %q, want %q", sc.Name, "test-scorecard")
	}
	if len(sc.MetricDefs) != 1 {
		t.Fatalf("got %d metrics, want 1", len(sc.MetricDefs))
	}
	m := sc.MetricDefs[0]
	if m.ID != "M1" {
		t.Errorf("ID = %q, want %q", m.ID, "M1")
	}
	if m.Name != "accuracy" {
		t.Errorf("Name = %q, want %q", m.Name, "accuracy")
	}
	if m.Threshold != 0.8 {
		t.Errorf("Threshold = %f, want 0.8", m.Threshold)
	}
	if m.Direction != calibrate.HigherIsBetter {
		t.Errorf("Direction = %q, want %q", m.Direction, calibrate.HigherIsBetter)
	}
	if m.Weight != 1.0 {
		t.Errorf("Weight = %f, want 1.0", m.Weight)
	}
}

func TestScoreCardBuilder_WithScorer(t *testing.T) {
	sc := builders.NewScoreCard().
		AddMetric("M1", "accuracy", 0.8).
		WithScorer("accuracy").
		AddMetric("M2", "recall", 0.7).
		Build()

	if len(sc.MetricDefs) != 2 {
		t.Fatalf("got %d metrics, want 2", len(sc.MetricDefs))
	}
	// M1 was added before WithScorer, should have empty scorer.
	if sc.MetricDefs[0].Scorer != "" {
		t.Errorf("M1.Scorer = %q, want empty", sc.MetricDefs[0].Scorer)
	}
	// M2 was added after WithScorer, should have "accuracy" scorer.
	if sc.MetricDefs[1].Scorer != "accuracy" {
		t.Errorf("M2.Scorer = %q, want %q", sc.MetricDefs[1].Scorer, "accuracy")
	}
}

func TestScoreCardBuilder_WithName(t *testing.T) {
	sc := builders.NewScoreCard().
		WithName("custom").
		AddMetric("M1", "accuracy", 0.8).
		Build()

	if sc.Name != "custom" {
		t.Errorf("Name = %q, want %q", sc.Name, "custom")
	}
}

func TestScoreCardBuilder_Evaluate(t *testing.T) {
	sc := builders.NewScoreCard().
		AddMetric("M1", "accuracy", 0.8).
		AddMetric("M2", "recall", 0.7).
		Build()

	ms := sc.Evaluate(
		map[string]float64{"M1": 0.9, "M2": 0.5},
		nil,
	)

	metrics := ms.AllMetrics()
	if len(metrics) != 2 {
		t.Fatalf("got %d metrics, want 2", len(metrics))
	}

	// M1: 0.9 >= 0.8 → pass
	if !metrics[0].Pass {
		t.Error("M1 should pass (0.9 >= 0.8)")
	}
	// M2: 0.5 < 0.7 → fail
	if metrics[1].Pass {
		t.Error("M2 should fail (0.5 < 0.7)")
	}
}

func TestScoreCardBuilder_Empty(t *testing.T) {
	sc := builders.NewScoreCard().Build()

	if sc.Version != 1 {
		t.Errorf("Version = %d, want 1", sc.Version)
	}
	if len(sc.MetricDefs) != 0 {
		t.Errorf("got %d metrics, want 0", len(sc.MetricDefs))
	}
}

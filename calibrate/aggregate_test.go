package calibrate_test

import (
	"math"
	"testing"

	"github.com/dpopsuev/origami/calibrate"
)

func TestAggregateRunMetrics_Empty(t *testing.T) {
	agg := calibrate.AggregateRunMetrics(nil, nil)
	if got := len(agg.AllMetrics()); got != 0 {
		t.Fatalf("empty: want 0 metrics, got %d", got)
	}
}

func TestAggregateRunMetrics_SingleRun(t *testing.T) {
	ms := sampleMetricSet()
	agg := calibrate.AggregateRunMetrics([]calibrate.MetricSet{ms}, nil)
	if got := len(agg.AllMetrics()); got != 8 {
		t.Fatalf("single: want 8 metrics, got %d", got)
	}
	if agg.Metrics[0].Value != 0.80 {
		t.Errorf("single: M1 value want 0.80, got %.2f", agg.Metrics[0].Value)
	}
}

func TestAggregateRunMetrics_TwoIdenticalRuns(t *testing.T) {
	ms := sampleMetricSet()
	agg := calibrate.AggregateRunMetrics([]calibrate.MetricSet{ms, ms}, nil)
	for _, m := range agg.AllMetrics() {
		orig := findMetric(sampleMetricSet(), m.ID)
		if math.Abs(m.Value-orig.Value) > 1e-9 {
			t.Errorf("%s: want %.4f, got %.4f", m.ID, orig.Value, m.Value)
		}
	}
}

func TestAggregateRunMetrics_Averaging(t *testing.T) {
	run1 := calibrate.MetricSet{
		Metrics: []calibrate.Metric{{ID: "X", Name: "x", Value: 0.60, Threshold: 0.50}},
	}
	run2 := calibrate.MetricSet{
		Metrics: []calibrate.Metric{{ID: "X", Name: "x", Value: 0.80, Threshold: 0.50}},
	}
	agg := calibrate.AggregateRunMetrics([]calibrate.MetricSet{run1, run2}, nil)
	if got := agg.Metrics[0].Value; math.Abs(got-0.70) > 1e-9 {
		t.Errorf("avg: want 0.70, got %.4f", got)
	}
	if !agg.Metrics[0].Pass {
		t.Error("avg: want pass=true (0.70 >= 0.50)")
	}
}

func TestAggregateRunMetrics_CustomEvaluator(t *testing.T) {
	run := calibrate.MetricSet{
		Metrics: []calibrate.Metric{{ID: "X", Value: 0.20, Threshold: 0.30}},
	}
	alwaysFail := func(_ *calibrate.Metric) bool { return false }
	agg := calibrate.AggregateRunMetrics([]calibrate.MetricSet{run, run}, alwaysFail)
	if agg.Metrics[0].Pass {
		t.Error("custom eval: want pass=false")
	}
}

func TestMean(t *testing.T) {
	tests := []struct {
		vals []float64
		want float64
	}{
		{nil, 0},
		{[]float64{}, 0},
		{[]float64{5}, 5},
		{[]float64{2, 4}, 3},
		{[]float64{1, 2, 3, 4, 5}, 3},
	}
	for _, tt := range tests {
		if got := calibrate.Mean(tt.vals); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("Mean(%v): want %.4f, got %.4f", tt.vals, tt.want, got)
		}
	}
}

func TestStddev(t *testing.T) {
	tests := []struct {
		vals []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 0},
		{[]float64{2, 4}, math.Sqrt(2)},
	}
	for _, tt := range tests {
		if got := calibrate.Stddev(tt.vals); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("Stddev(%v): want %.4f, got %.4f", tt.vals, tt.want, got)
		}
	}
}

func TestSafeDiv(t *testing.T) {
	if got := calibrate.SafeDiv(3, 4); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("SafeDiv(3,4): want 0.75, got %.4f", got)
	}
	if got := calibrate.SafeDiv(0, 0); got != 1.0 {
		t.Errorf("SafeDiv(0,0): want 1.0, got %.4f", got)
	}
}

func TestSafeDivFloat(t *testing.T) {
	if got := calibrate.SafeDivFloat(1.5, 3.0); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("SafeDivFloat(1.5,3.0): want 0.5, got %.4f", got)
	}
	if got := calibrate.SafeDivFloat(0, 0); got != 1.0 {
		t.Errorf("SafeDivFloat(0,0): want 1.0, got %.4f", got)
	}
}

func findMetric(ms calibrate.MetricSet, id string) calibrate.Metric {
	for _, m := range ms.AllMetrics() {
		if m.ID == id {
			return m
		}
	}
	return calibrate.Metric{}
}

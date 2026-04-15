package calibrate

import (
	"fmt"
	"math"
)

// PassEvaluator decides whether a metric passes after aggregation.
// Consumers provide domain-specific logic (e.g. "M4 uses <= instead of >=").
type PassEvaluator func(*Metric) bool

// DefaultPassEvaluator returns true when the metric value meets or exceeds
// its threshold. Suitable for "higher is better" metrics.
func DefaultPassEvaluator(m *Metric) bool {
	return m.Value >= m.Threshold
}

// AggregateRunMetrics computes the mean value for each metric across
// multiple runs and re-evaluates pass/fail using the provided evaluator.
// All metrics are averaged. Consumers that need special aggregate handling
// (e.g. variance metrics) should post-process the returned MetricSet.
//
// If eval is nil, DefaultPassEvaluator is used.
func AggregateRunMetrics(runs []MetricSet, eval PassEvaluator) MetricSet {
	if len(runs) == 0 {
		return MetricSet{}
	}
	if len(runs) == 1 {
		return runs[0]
	}
	if eval == nil {
		eval = DefaultPassEvaluator
	}

	allByID := make(map[string][]float64)
	for _, run := range runs {
		for _, m := range run.AllMetrics() {
			allByID[m.ID] = append(allByID[m.ID], m.Value)
		}
	}

	agg := runs[0]
	for i := range agg.Metrics {
		vals := allByID[agg.Metrics[i].ID]
		agg.Metrics[i].Value = Mean(vals)
		sd := Stddev(vals)
		agg.Metrics[i].Detail = fmt.Sprintf("mean of %d runs (σ=%.3f)", len(runs), sd)
		agg.Metrics[i].Pass = eval(&agg.Metrics[i])
	}

	return agg
}

// Mean returns the arithmetic mean of vals. Returns 0 for empty input.
func Mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// Stddev returns the sample standard deviation (Bessel-corrected, N-1).
// Returns 0 when fewer than 2 values are provided.
func Stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := Mean(vals)
	sum := 0.0
	for _, v := range vals {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

// SafeDiv divides two integers. Returns 1.0 when denom is 0.
func SafeDiv(num, denom int) float64 {
	if denom == 0 {
		return 1.0
	}
	return float64(num) / float64(denom)
}

// SafeDivFloat divides two float64 values. Returns 1.0 when denom is 0.
func SafeDivFloat(num, denom float64) float64 {
	if denom == 0 {
		return 1.0
	}
	return num / denom
}

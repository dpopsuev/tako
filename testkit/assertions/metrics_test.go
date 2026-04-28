package assertions_test

import (
	"testing"

	"github.com/dpopsuev/tako/calibrate"
	"github.com/dpopsuev/tako/testkit/assertions"
)

func buildReport(metrics ...calibrate.Metric) *calibrate.CalibrationReport {
	return &calibrate.CalibrationReport{
		Scenario:    "test",
		Transformer: "stub",
		Runs:        1,
		Metrics:     calibrate.MetricSet{Metrics: metrics},
	}
}

func TestAssertMetricPassed(t *testing.T) {
	report := buildReport(
		calibrate.Metric{ID: "M1", Value: 0.9, Threshold: 0.8, Pass: true, Direction: calibrate.HigherIsBetter},
		calibrate.Metric{ID: "M2", Value: 0.5, Threshold: 0.7, Pass: false, Direction: calibrate.HigherIsBetter},
	)

	assertions.AssertMetricPassed(t, report, "M1")
}

func TestAssertMetricFailed(t *testing.T) {
	report := buildReport(
		calibrate.Metric{ID: "M1", Value: 0.9, Threshold: 0.8, Pass: true, Direction: calibrate.HigherIsBetter},
		calibrate.Metric{ID: "M2", Value: 0.5, Threshold: 0.7, Pass: false, Direction: calibrate.HigherIsBetter},
	)

	assertions.AssertMetricFailed(t, report, "M2")
}

func TestAssertMetricPassed_WithScoreCard(t *testing.T) {
	sc := calibrate.ScoreCard{
		Name:    "test",
		Version: 1,
		MetricDefs: []calibrate.MetricDef{
			{ID: "accuracy", Name: "Accuracy", Threshold: 0.8, Direction: calibrate.HigherIsBetter},
			{ID: "cost", Name: "Cost", Threshold: 5.0, Direction: calibrate.LowerIsBetter},
		},
	}

	ms := sc.Evaluate(
		map[string]float64{"accuracy": 0.95, "cost": 3.0},
		nil,
	)
	report := &calibrate.CalibrationReport{
		Scenario: "test",
		Runs:     1,
		Metrics:  ms,
	}

	assertions.AssertMetricPassed(t, report, "accuracy")
	assertions.AssertMetricPassed(t, report, "cost")
}

func TestAssertMetricFailed_LowerIsBetter(t *testing.T) {
	sc := calibrate.ScoreCard{
		Name:    "test",
		Version: 1,
		MetricDefs: []calibrate.MetricDef{
			{ID: "cost", Name: "Cost", Threshold: 5.0, Direction: calibrate.LowerIsBetter},
		},
	}

	ms := sc.Evaluate(
		map[string]float64{"cost": 10.0},
		nil,
	)
	report := &calibrate.CalibrationReport{
		Scenario: "test",
		Runs:     1,
		Metrics:  ms,
	}

	assertions.AssertMetricFailed(t, report, "cost")
}

package assertions

import (
	"testing"

	"github.com/dpopsuev/origami/calibrate"
)

// AssertMetricPassed verifies that the metric with the given ID in the
// CalibrationReport has Pass=true.
func AssertMetricPassed(t testing.TB, report *calibrate.CalibrationReport, metricID string) {
	t.Helper()

	m, found := findMetric(report, metricID)
	if !found {
		t.Errorf("metric %q not found in report", metricID)
		return
	}
	if !m.Pass {
		t.Errorf("metric %q failed (value=%.4f, threshold=%.4f, direction=%s)",
			metricID, m.Value, m.Threshold, m.Direction)
	}
}

// AssertMetricFailed verifies that the metric with the given ID in the
// CalibrationReport has Pass=false.
func AssertMetricFailed(t testing.TB, report *calibrate.CalibrationReport, metricID string) {
	t.Helper()

	m, found := findMetric(report, metricID)
	if !found {
		t.Errorf("metric %q not found in report", metricID)
		return
	}
	if m.Pass {
		t.Errorf("metric %q passed but expected failure (value=%.4f, threshold=%.4f, direction=%s)",
			metricID, m.Value, m.Threshold, m.Direction)
	}
}

func findMetric(report *calibrate.CalibrationReport, id string) (calibrate.Metric, bool) {
	for _, m := range report.Metrics.Metrics {
		if m.ID == id {
			return m, true
		}
	}
	return calibrate.Metric{}, false
}

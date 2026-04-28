package assertions

import (
	"testing"

	"github.com/dpopsuev/tako/calibrate"
)

// AssertMetricPassed verifies that the metric with the given ID in the
// CalibrationReport has Pass=true.
func AssertMetricPassed(tb testing.TB, report *calibrate.CalibrationReport, metricID string) {
	tb.Helper()

	m, found := findMetric(report, metricID)
	if !found {
		tb.Errorf("metric %q not found in report", metricID)
		return
	}
	if !m.Pass {
		tb.Errorf("metric %q failed (value=%.4f, threshold=%.4f, direction=%s)",
			metricID, m.Value, m.Threshold, m.Direction)
	}
}

// AssertMetricFailed verifies that the metric with the given ID in the
// CalibrationReport has Pass=false.
func AssertMetricFailed(tb testing.TB, report *calibrate.CalibrationReport, metricID string) {
	tb.Helper()

	m, found := findMetric(report, metricID)
	if !found {
		tb.Errorf("metric %q not found in report", metricID)
		return
	}
	if m.Pass {
		tb.Errorf("metric %q passed but expected failure (value=%.4f, threshold=%.4f, direction=%s)",
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

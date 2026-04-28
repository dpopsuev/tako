package calibrate_test

import (
	"strings"
	"testing"

	"github.com/dpopsuev/tako/budget"
	"github.com/dpopsuev/tako/calibrate"
)

func testReport() *calibrate.CalibrationReport {
	return &calibrate.CalibrationReport{
		Scenario:    "unit-test",
		Transformer: "stub",
		Runs:        1,
		Metrics: calibrate.MetricSet{
			Metrics: []calibrate.Metric{
				{ID: "M1", Name: "Accuracy", Value: 0.80, Threshold: 0.80, Pass: true, Tier: calibrate.TierOutcome},
				{ID: "M2", Name: "Coverage", Value: 0.50, Threshold: 0.75, Pass: false, Tier: calibrate.TierOutcome},
				{ID: "M9", Name: "Workspace", Value: 1.00, Threshold: 0.70, Pass: true, Tier: calibrate.TierInvestigation},
			},
		},
	}
}

func TestFormatReport_ContainsMetrics(t *testing.T) {
	out := calibrate.FormatReport(testReport(), calibrate.FormatConfig{})
	if !strings.Contains(out, "M1") {
		t.Error("report should contain M1")
	}
	if !strings.Contains(out, "M2") {
		t.Error("report should contain M2")
	}
}

func TestFormatReport_Title(t *testing.T) {
	out := calibrate.FormatReport(testReport(), calibrate.FormatConfig{Title: "Custom Title"})
	if !strings.Contains(out, "Custom Title") {
		t.Error("custom title missing")
	}
}

func TestFormatReport_Scenario(t *testing.T) {
	out := calibrate.FormatReport(testReport(), calibrate.FormatConfig{})
	if !strings.Contains(out, "unit-test") {
		t.Error("scenario missing")
	}
}

func TestFormatReport_PassFail(t *testing.T) {
	out := calibrate.FormatReport(testReport(), calibrate.FormatConfig{})
	if !strings.Contains(out, "FAIL") {
		t.Error("should contain FAIL when not all metrics pass")
	}
}

func TestFormatReport_AllPass(t *testing.T) {
	r := &calibrate.CalibrationReport{
		Scenario:    "pass-test",
		Transformer: "stub",
		Runs:        1,
		Metrics: calibrate.MetricSet{
			Metrics: []calibrate.Metric{
				{ID: "M1", Name: "A", Value: 0.90, Threshold: 0.80, Pass: true, Tier: calibrate.TierOutcome},
			},
		},
	}
	out := calibrate.FormatReport(r, calibrate.FormatConfig{})
	if !strings.Contains(out, "PASS (1/1") {
		t.Errorf("should contain PASS (1/1), got: %s", out)
	}
}

func TestFormatReport_WithExplicitSections(t *testing.T) {
	r := testReport()
	cfg := calibrate.FormatConfig{
		Sections: []calibrate.MetricSection{
			{Title: "Custom Section", Metrics: r.Metrics.Metrics},
		},
	}
	out := calibrate.FormatReport(r, cfg)
	if !strings.Contains(out, "Custom Section") {
		t.Error("explicit section title missing")
	}
}

func TestFormatReport_AutoSectionsFromTier(t *testing.T) {
	out := calibrate.FormatReport(testReport(), calibrate.FormatConfig{})
	if !strings.Contains(out, "Outcome") {
		t.Error("auto-generated Outcome section missing")
	}
	if !strings.Contains(out, "Investigation") {
		t.Error("auto-generated Investigation section missing")
	}
}

func TestFormatReport_WithTokens(t *testing.T) {
	r := testReport()
	r.Tokens = &budget.TokenSummary{
		TotalPromptTokens:   1000,
		TotalArtifactTokens: 500,
	}
	out := calibrate.FormatReport(r, calibrate.FormatConfig{})
	if !strings.Contains(out, "1000") {
		t.Error("token summary missing")
	}
}

func TestFormatReport_DryCappedMark(t *testing.T) {
	r := &calibrate.CalibrationReport{
		Scenario:    "dry-test",
		Transformer: "stub",
		Runs:        1,
		Metrics: calibrate.MetricSet{
			Metrics: []calibrate.Metric{
				{ID: "M12", Name: "Evidence", Value: 0.60, Threshold: 0.60, Pass: true, DryCapped: true, Tier: calibrate.TierDetection},
			},
		},
	}
	out := calibrate.FormatReport(r, calibrate.FormatConfig{})
	if !strings.Contains(out, "~") {
		t.Error("dry-capped should show ~ mark")
	}
	// DryCapped is excluded from PassCount
	if !strings.Contains(out, "0/0") {
		t.Errorf("dry-capped metric excluded from count, want 0/0, got: %s", out)
	}
}

func TestFormatReport_Empty(t *testing.T) {
	r := &calibrate.CalibrationReport{
		Scenario:    "empty",
		Transformer: "stub",
		Runs:        0,
		Metrics:     calibrate.MetricSet{},
	}
	out := calibrate.FormatReport(r, calibrate.FormatConfig{})
	if !strings.Contains(out, "PASS (0/0") {
		t.Errorf("empty should be PASS 0/0, got: %s", out)
	}
}

func TestFormatReport_MetricNameFunc(t *testing.T) {
	r := testReport()
	cfg := calibrate.FormatConfig{
		MetricNameFunc: func(id string) string {
			if id == "M1" {
				return "Custom Accuracy"
			}
			return ""
		},
	}
	out := calibrate.FormatReport(r, cfg)
	if !strings.Contains(out, "Custom Accuracy") {
		t.Error("MetricNameFunc output missing")
	}
}

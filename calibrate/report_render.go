package calibrate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MarkdownReportRenderer renders a CalibrationReport as a Markdown table.
type MarkdownReportRenderer struct{}

func (r *MarkdownReportRenderer) Render(report *CalibrationReport) (string, error) {
	if report == nil {
		return "", fmt.Errorf("nil report")
	}
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Calibration Report: %s\n\n", report.Scenario))
	sb.WriteString(fmt.Sprintf("- **Transformer:** %s\n", report.Transformer))
	if report.Resolution != "" {
		sb.WriteString(fmt.Sprintf("- **Resolution:** %s\n", report.Resolution))
	}
	sb.WriteString(fmt.Sprintf("- **Runs:** %d\n\n", report.Runs))

	// Metrics table
	passed, total := report.Metrics.PassCount()
	sb.WriteString(fmt.Sprintf("## Metrics (%d/%d passed)\n\n", passed, total))
	sb.WriteString("| ID | Metric | Value | Threshold | Pass |\n")
	sb.WriteString("|-----|--------|-------|-----------|------|\n")
	for _, m := range report.Metrics.Metrics {
		pass := "PASS"
		if !m.Pass {
			pass = "FAIL"
		}
		if m.DryCapped {
			pass = "DRY"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %s |\n",
			m.ID, m.Name, m.Value, m.Threshold, pass))
	}

	// Token summary
	if report.Tokens != nil {
		sb.WriteString(fmt.Sprintf("\n## Tokens\n\n- Prompt: %d\n- Artifact: %d\n- Total: %d\n- Cost: $%.4f\n",
			report.Tokens.TotalPromptTokens, report.Tokens.TotalArtifactTokens,
			report.Tokens.TotalTokens, report.Tokens.TotalCostUSD))
	}

	// Multi-run breakdown
	if len(report.RunMetrics) > 1 {
		sb.WriteString("\n## Per-Run Breakdown\n\n")
		for i, rm := range report.RunMetrics {
			p, t := rm.PassCount()
			sb.WriteString(fmt.Sprintf("- Run %d: %d/%d passed\n", i+1, p, t))
		}
	}

	return sb.String(), nil
}

// JSONReportRenderer renders a CalibrationReport as pretty-printed JSON.
type JSONReportRenderer struct{}

func (r *JSONReportRenderer) Render(report *CalibrationReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	return string(data), nil
}

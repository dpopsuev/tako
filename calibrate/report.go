package calibrate

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/format"
)

const (
	markPass = "✓"
	markFail = "✗"
)

// MetricSection groups metrics under a titled section for report formatting.
type MetricSection struct {
	Title   string
	Metrics []Metric
}

// FormatConfig controls how FormatReport renders the calibration report.
type FormatConfig struct {
	Title          string
	Sections       []MetricSection
	MetricNameFunc func(id string) string
	ThresholdFunc  func(*Metric) string
}

// DefaultThresholdFormat renders the threshold as ">= X.XX".
func DefaultThresholdFormat(m *Metric) string {
	return fmt.Sprintf("≥%.2f", m.Threshold)
}

// tierOrder defines the display ordering for auto-generated sections.
var tierOrder = []CostTier{TierOutcome, TierInvestigation, TierDetection, TierEfficiency, TierMeta, ""}

// tierTitle maps a CostTier to a human-readable section title.
var tierTitle = map[CostTier]string{
	TierOutcome:       "Outcome",
	TierInvestigation: "Investigation",
	TierDetection:     "Detection",
	TierEfficiency:    "Efficiency",
	TierMeta:          "Meta",
	"":                "Other",
}

// sectionsFromTier auto-generates MetricSections grouped by Tier.
func sectionsFromTier(ms MetricSet) []MetricSection {
	byTier := ms.ByTier()
	sections := make([]MetricSection, 0, len(tierOrder))
	for _, tier := range tierOrder {
		metrics, ok := byTier[tier]
		if !ok || len(metrics) == 0 {
			continue
		}
		title := tierTitle[tier]
		sections = append(sections, MetricSection{Title: title, Metrics: metrics})
	}
	return sections
}

// FormatReport produces a human-readable calibration report with metric
// tables, a pass/fail result line, and an optional token summary.
// When cfg.Sections is empty, sections are auto-generated from Metric.Tier.
func FormatReport(report *CalibrationReport, cfg FormatConfig) string {
	var b strings.Builder

	title := cfg.Title
	if title == "" {
		title = "Calibration Report"
	}
	b.WriteString(fmt.Sprintf("=== %s ===\n", title))
	b.WriteString(fmt.Sprintf("Scenario: %s\n", report.Scenario))
	b.WriteString(fmt.Sprintf("Transformer: %s\n", report.Transformer))
	if report.Resolution != "" {
		b.WriteString(fmt.Sprintf("Resolution: %s\n", report.Resolution))
	}
	if report.Plan != "" {
		b.WriteString(fmt.Sprintf("Plan: %s\n", report.Plan))
	}
	b.WriteString(fmt.Sprintf("Runs:     %d\n\n", report.Runs))

	nameFunc := cfg.MetricNameFunc
	if nameFunc == nil {
		nameFunc = func(_ string) string { return "" }
	}
	threshFunc := cfg.ThresholdFunc
	if threshFunc == nil {
		threshFunc = DefaultThresholdFormat
	}

	sections := cfg.Sections
	if len(sections) == 0 {
		sections = sectionsFromTier(report.Metrics)
	}

	for _, sec := range sections {
		b.WriteString(fmt.Sprintf("--- %s ---\n", sec.Title))
		tbl := format.NewTable(format.ASCII)
		tbl.Header("ID", "Metric", "Value", "Detail", "Pass", "Threshold")
		tbl.Columns(
			format.ColumnConfig{Number: 1, Align: format.AlignLeft},
			format.ColumnConfig{Number: 2, Align: format.AlignLeft},
			format.ColumnConfig{Number: 3, Align: format.AlignRight},
			format.ColumnConfig{Number: 4, Align: format.AlignLeft},
			format.ColumnConfig{Number: 5, Align: format.AlignCenter},
			format.ColumnConfig{Number: 6, Align: format.AlignLeft},
		)
		for _, m := range sec.Metrics {
			displayName := nameFunc(m.ID)
			if displayName == "" {
				displayName = m.Name
			}
			passMark := format.BoolMark(m.Pass)
			if m.DryCapped {
				passMark = "~"
			}
			tbl.Row(
				m.ID,
				displayName,
				fmt.Sprintf("%.2f", m.Value),
				m.Detail,
				passMark,
				threshFunc(&m),
			)
		}
		b.WriteString(tbl.String())
		b.WriteString("\n\n")
	}

	passed, total := report.Metrics.PassCount()
	result := "PASS"
	if passed < total {
		result = "FAIL"
	}
	b.WriteString(fmt.Sprintf("RESULT: %s (%d/%d metrics within threshold)\n\n", result, passed, total))

	if report.Tokens != nil {
		b.WriteString(dispatch.FormatTokenSummary(*report.Tokens))
		b.WriteString("\n")
	}

	return b.String()
}

// ResolutionComparison holds metric deltas between two resolution levels.
type ResolutionComparison struct {
	MetricID   string  `json:"metric_id"`
	MetricName string  `json:"metric_name"`
	ValueA     float64 `json:"value_a"`
	ValueB     float64 `json:"value_b"`
	Delta      float64 `json:"delta"`
	PassA      bool    `json:"pass_a"`
	PassB      bool    `json:"pass_b"`
}

// CompareResolutions produces a metric-by-metric comparison between two
// calibration reports at different resolution levels. This surfaces which
// metrics degrade when moving from isolated to integrated calibration.
func CompareResolutions(a, b *CalibrationReport) []ResolutionComparison {
	bMap := b.Metrics.ByID()
	comps := make([]ResolutionComparison, 0, len(a.Metrics.Metrics))
	for _, ma := range a.Metrics.Metrics {
		comp := ResolutionComparison{
			MetricID:   ma.ID,
			MetricName: ma.Name,
			ValueA:     ma.Value,
			PassA:      ma.Pass,
		}
		if mb, ok := bMap[ma.ID]; ok {
			comp.ValueB = mb.Value
			comp.PassB = mb.Pass
			comp.Delta = mb.Value - ma.Value
		}
		comps = append(comps, comp)
	}
	return comps
}

// FormatResolutionComparison renders a cross-resolution comparison table.
func FormatResolutionComparison(comps []ResolutionComparison, labelA, labelB string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== Resolution Comparison: %s vs %s ===\n\n", labelA, labelB))

	tbl := format.NewTable(format.ASCII)
	tbl.Header("ID", "NAME", labelA, "", labelB, "", "DELTA")

	for _, c := range comps {
		passA := markPass
		if !c.PassA {
			passA = markFail
		}
		passB := markPass
		if !c.PassB {
			passB = markFail
		}
		tbl.Row(
			c.MetricID, c.MetricName,
			fmt.Sprintf("%.3f", c.ValueA), passA,
			fmt.Sprintf("%.3f", c.ValueB), passB,
			fmt.Sprintf("%+.3f", c.Delta),
		)
	}

	b.WriteString(tbl.String())
	return b.String()
}

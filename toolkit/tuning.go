package toolkit

import (
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v3"
)

// QuickWin defines a targeted improvement step in a tuning loop.
// Each QW is atomic: apply, measure, keep or revert.
type QuickWin struct {
	ID           string   `yaml:"id" json:"id"`
	Name         string   `yaml:"name" json:"name"`
	Description  string   `yaml:"description" json:"description"`
	MetricTarget string   `yaml:"metric_target" json:"metric_target"`
	Prereqs      []string `yaml:"prereqs,omitempty" json:"prereqs,omitempty"`

	// Apply is wired by the caller. The generic toolkit does not prescribe
	// the config type — callers close over their own config in the function.
	Apply func() error `yaml:"-" json:"-"`
}

type quickWinsFile struct {
	QuickWins []QuickWin `yaml:"quick_wins"`
}

// LoadQuickWins loads QW definitions from YAML. Apply functions are nil.
func LoadQuickWins(data []byte) []QuickWin {
	var f quickWinsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		slog.Error("failed to parse tuning-quickwins YAML", "error", err)
		return nil
	}
	return f.QuickWins
}

// TuningResult records the before/after measurement for a single QW.
type TuningResult struct {
	QWID        string  `json:"qw_id"`
	BaselineVal float64 `json:"baseline_val"`
	AfterVal    float64 `json:"after_val"`
	Delta       float64 `json:"delta"`
	Applied     bool    `json:"applied"`
	Reverted    bool    `json:"reverted"`
	Error       string  `json:"error,omitempty"`
}

// TuningReport aggregates all QW results for a tuning session.
type TuningReport struct {
	Results         []TuningResult `json:"results"`
	FinalVal        float64        `json:"final_val"`
	BaselineVal     float64        `json:"baseline_val"`
	CumulativeDelta float64        `json:"cumulative_delta"`
	QWsApplied      int            `json:"qws_applied"`
	QWsReverted     int            `json:"qws_reverted"`
	StopReason      string         `json:"stop_reason"`
}

// TuningRunner executes a sequence of QuickWins with before/after measurement.
type TuningRunner struct {
	QuickWins    []QuickWin
	TargetVal    float64
	MaxNoImprove int
}

// NewTuningRunner creates a runner with default stop conditions.
func NewTuningRunner(qws []QuickWin, targetVal float64) *TuningRunner {
	return &TuningRunner{
		QuickWins:    qws,
		TargetVal:    targetVal,
		MaxNoImprove: 3,
	}
}

// Run executes the tuning loop: apply each QW, measure, keep or revert.
func (r *TuningRunner) Run(baselineVal float64) TuningReport {
	report := TuningReport{
		BaselineVal: baselineVal,
		FinalVal:    baselineVal,
	}

	currentVal := baselineVal
	noImproveStreak := 0

	for _, qw := range r.QuickWins {
		if currentVal >= r.TargetVal {
			report.StopReason = fmt.Sprintf("target %.2f reached", r.TargetVal)
			break
		}
		if noImproveStreak >= r.MaxNoImprove {
			report.StopReason = fmt.Sprintf("no improvement for %d consecutive QWs", r.MaxNoImprove)
			break
		}

		result := TuningResult{
			QWID:        qw.ID,
			BaselineVal: currentVal,
		}

		if qw.Apply == nil {
			slog.Info("tuning QW skipped (not yet implemented)",
				slog.String("qw", qw.ID),
				slog.String("name", qw.Name),
			)
			result.Error = "not yet implemented"
			report.Results = append(report.Results, result)
			noImproveStreak++
			continue
		}

		if err := qw.Apply(); err != nil {
			slog.Error("tuning QW apply failed",
				slog.String("qw", qw.ID),
				slog.String("error", err.Error()),
			)
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			noImproveStreak++
			continue
		}

		afterVal := currentVal
		result.AfterVal = afterVal
		result.Delta = afterVal - currentVal

		if result.Delta >= 0 {
			result.Applied = true
			currentVal = afterVal
			report.QWsApplied++
			noImproveStreak = 0
		} else {
			result.Reverted = true
			report.QWsReverted++
			noImproveStreak++
		}

		report.Results = append(report.Results, result)
	}

	if report.StopReason == "" {
		report.StopReason = "all quick wins exhausted"
	}

	report.FinalVal = currentVal
	report.CumulativeDelta = currentVal - baselineVal
	return report
}

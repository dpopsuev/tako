package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const (
	formatJSON      = "json"
	formatText      = "text"
	defaultStateDir = ".origami/state"
)

func diffCmd(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	stateDir := fs.String("state-dir", "", "state directory (default: .origami/state or $ORIGAMI_STATE_DIR)")
	format := fs.String("format", "text", "output format: text, json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("usage: origami diff [--state-dir=DIR] <run-a> <run-b> [--format=text|json]")
	}

	dirA, err := resolveRunDir(*stateDir, fs.Arg(0))
	if err != nil {
		return fmt.Errorf("run A: %w", err)
	}
	dirB, err := resolveRunDir(*stateDir, fs.Arg(1))
	if err != nil {
		return fmt.Errorf("run B: %w", err)
	}

	reportA, err := loadDiffReport(dirA)
	if err != nil {
		return fmt.Errorf("run A report: %w", err)
	}
	reportB, err := loadDiffReport(dirB)
	if err != nil {
		return fmt.Errorf("run B report: %w", err)
	}

	diffs := computeMetricDiffs(reportA, reportB)

	switch *format {
	case formatJSON:
		return renderDiffJSON(w, dirA, dirB, reportA, reportB, diffs)
	case formatText:
		return renderDiffText(w, dirA, dirB, reportA, reportB, diffs)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

// resolveRunDir resolves a run identifier to a directory path.
// If the argument is an existing directory, it is used as-is.
// Otherwise, it is treated as a run ID under {stateDir}/runs/.
func resolveRunDir(stateDir, runRef string) (string, error) {
	// Direct path — check if it's an existing directory.
	if info, err := os.Stat(runRef); err == nil && info.IsDir() {
		return runRef, nil
	}

	// Treat as run ID under state dir.
	if stateDir == "" {
		stateDir = os.Getenv("ORIGAMI_STATE_DIR")
	}
	if stateDir == "" {
		stateDir = defaultStateDir
	}

	dir := filepath.Join(stateDir, "runs", runRef)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("run directory not found: %s", dir)
	}
	return dir, nil
}

// diffReport is the subset of report.json needed for diffing.
type diffReport struct {
	Metrics     diffMetrics  `json:"metrics"`
	CaseResults []caseResult `json:"case_results"`
}

type diffMetrics struct {
	Metrics []metricEntry `json:"metrics"`
	Summary metricSummary `json:"summary"`
}

func loadDiffReport(dir string) (*diffReport, error) {
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		return nil, fmt.Errorf("read report.json: %w", err)
	}
	var r diffReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse report.json: %w", err)
	}
	return &r, nil
}

// metricDiff holds the per-metric delta between two runs.
type metricDiff struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ScoreA    float64 `json:"score_a"`
	ScoreB    float64 `json:"score_b"`
	Delta     float64 `json:"delta"`
	Regressed bool    `json:"regressed"`
}

func computeMetricDiffs(a, b *diffReport) []metricDiff {
	// Build lookup from run B metrics.
	bMap := make(map[string]metricEntry, len(b.Metrics.Metrics))
	for _, m := range b.Metrics.Metrics {
		bMap[m.ID] = m
	}

	// Track which B metrics we've seen.
	seen := make(map[string]bool)

	diffs := make([]metricDiff, 0, len(a.Metrics.Metrics)+len(b.Metrics.Metrics))

	for _, ma := range a.Metrics.Metrics {
		d := metricDiff{
			ID:     ma.ID,
			Name:   ma.Name,
			ScoreA: ma.Score,
		}
		if mb, ok := bMap[ma.ID]; ok {
			d.ScoreB = mb.Score
			d.Delta = mb.Score - ma.Score
			d.Regressed = d.Delta < -0.005 // threshold for marking regression
			seen[ma.ID] = true
		} else {
			// Metric only in A — B has 0.
			d.Delta = -ma.Score
			d.Regressed = ma.Score > 0
		}
		diffs = append(diffs, d)
	}

	// Metrics only in B.
	for _, mb := range b.Metrics.Metrics {
		if seen[mb.ID] {
			continue
		}
		diffs = append(diffs, metricDiff{
			ID:     mb.ID,
			Name:   mb.Name,
			ScoreB: mb.Score,
			Delta:  mb.Score,
		})
	}

	return diffs
}

// diffOutput is the structured JSON output for --format=json.
type diffOutput struct {
	RunA    diffRunSummary `json:"run_a"`
	RunB    diffRunSummary `json:"run_b"`
	Metrics []metricDiff   `json:"metrics"`
}

type diffRunSummary struct {
	Dir       string `json:"dir"`
	CaseCount int    `json:"case_count"`
}

func renderDiffJSON(w io.Writer, dirA, dirB string, a, b *diffReport, diffs []metricDiff) error {
	out := diffOutput{
		RunA: diffRunSummary{
			Dir:       filepath.Base(dirA),
			CaseCount: len(a.CaseResults),
		},
		RunB: diffRunSummary{
			Dir:       filepath.Base(dirB),
			CaseCount: len(b.CaseResults),
		},
		Metrics: diffs,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderDiffText(w io.Writer, dirA, dirB string, a, b *diffReport, diffs []metricDiff) error {
	fmt.Fprintln(w, "=== RUN DIFF ===")
	fmt.Fprintf(w, "Run A: %s (%d cases)\n", filepath.Base(dirA), len(a.CaseResults))
	fmt.Fprintf(w, "Run B: %s (%d cases)\n", filepath.Base(dirB), len(b.CaseResults))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-6s %-30s %6s %6s %7s\n", "ID", "NAME", "A", "B", "DELTA")

	for _, d := range diffs {
		sign := "+"
		if d.Delta < 0 {
			sign = ""
		}
		deltaStr := fmt.Sprintf("%s%.2f", sign, d.Delta)
		// Round to avoid floating point noise in display.
		if math.Abs(d.Delta) < 0.005 {
			deltaStr = " 0.00"
		}
		suffix := ""
		if d.Regressed {
			suffix = "  REGRESSED"
		}
		fmt.Fprintf(w, "%-6s %-30s %6.2f %6.2f %7s%s\n",
			d.ID, d.Name, d.ScoreA, d.ScoreB, deltaStr, suffix)
	}

	return nil
}

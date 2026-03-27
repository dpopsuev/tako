package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func reportCmd(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	stateDir := fs.String("state-dir", "", "state directory (default: .origami/state or $ORIGAMI_STATE_DIR)")
	runID := fs.String("run", "", "run ID (default: most recent)")
	format := fs.String("format", "text", "output format: text, json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reportPath, err := resolveReportPath(*stateDir, *runID, fs.Arg(0))
	if err != nil {
		return err
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read report: %w", err)
	}

	switch *format {
	case "json":
		// Pass through raw JSON.
		_, err := w.Write(data)
		return err
	case "text":
		return renderReportText(w, data)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

func resolveReportPath(stateDir, runID, fileArg string) (string, error) {
	if fileArg != "" {
		return fileArg, nil
	}

	if stateDir == "" {
		stateDir = os.Getenv("ORIGAMI_STATE_DIR")
	}
	if stateDir == "" {
		stateDir = ".origami/state"
	}

	runsDir := filepath.Join(stateDir, "runs")

	if runID == "" {
		entries, err := os.ReadDir(runsDir)
		if err != nil {
			return "", fmt.Errorf("cannot read runs directory %s: %w", runsDir, err)
		}
		var dirs []os.DirEntry
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e)
			}
		}
		if len(dirs) == 0 {
			return "", fmt.Errorf("no runs found in %s", runsDir)
		}
		sort.Slice(dirs, func(i, j int) bool {
			fi, _ := dirs[i].Info()
			fj, _ := dirs[j].Info()
			return fi.ModTime().After(fj.ModTime())
		})
		runID = dirs[0].Name()
	}

	return filepath.Join(runsDir, runID, "report.json"), nil
}

// reportData is the top-level structure of a report.json file.
type reportData struct {
	Metrics     reportMetrics   `json:"metrics"`
	CaseResults []caseResult    `json:"case_results"`
}

type reportMetrics struct {
	Metrics []metricEntry  `json:"metrics"`
	Summary metricSummary  `json:"summary"`
}

type metricEntry struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Passed    bool    `json:"passed"`
	Threshold float64 `json:"threshold"`
}

type metricSummary struct {
	Passed int     `json:"passed"`
	Total  int     `json:"total"`
	Score  float64 `json:"score"`
}

type caseResult struct {
	CaseID             string  `json:"case_id"`
	Version            string  `json:"version"`
	Job                string  `json:"job"`
	ComponentCorrect   bool    `json:"component_correct"`
	DefectTypeCorrect  bool    `json:"defect_type_correct"`
	CategoryCorrect    bool    `json:"category_correct"`
	PathCorrect        bool    `json:"path_correct"`
	StepCount          int     `json:"step_count"`
	ActualConvergence  float64 `json:"actual_convergence"`
	CircuitError       string  `json:"circuit_error"`
}

func renderReportText(w io.Writer, data []byte) error {
	var report reportData
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse report: %w", err)
	}

	// Scorecard.
	fmt.Fprintln(w, "=== SCORECARD ===")
	fmt.Fprintf(w, "%-6s %-30s %6s  %6s  %4s\n", "ID", "NAME", "SCORE", "THRESH", "PASS")
	for _, m := range report.Metrics.Metrics {
		pass := "N"
		if m.Passed {
			pass = "Y"
		}
		fmt.Fprintf(w, "%-6s %-30s %6.2f  %6.2f    %s\n", m.ID, m.Name, m.Score, m.Threshold, pass)
	}
	fmt.Fprintf(w, "Passed: %d/%d  Score: %.2f\n", report.Metrics.Summary.Passed, report.Metrics.Summary.Total, report.Metrics.Summary.Score)

	// Cases.
	if len(report.CaseResults) > 0 {
		renderCaseResults(w, report.CaseResults)
	}

	return nil
}

func renderCaseResults(w io.Writer, cases []caseResult) {
	fmt.Fprintf(w, "\n=== CASES (%d) ===\n", len(cases))
	fmt.Fprintf(w, "%-6s %-7s %-10s %4s  %5s  %3s  %5s  %4s  %s\n",
		"CASE", "VER", "JOB", "COMP", "DEFCT", "CAT", "STEPS", "CONV", "ERROR")

	var compOK, defOK, catOK int
	for _, c := range cases {
		if c.ComponentCorrect {
			compOK++
		}
		if c.DefectTypeCorrect {
			defOK++
		}
		if c.CategoryCorrect {
			catOK++
		}
		renderCaseRow(w, c)
	}

	total := len(cases)
	pct := func(n int) int {
		if total == 0 {
			return 0
		}
		return n * 100 / total
	}

	fmt.Fprintf(w, "\nComponent: %d/%d (%d%%)  Defect: %d/%d (%d%%)  Category: %d/%d (%d%%)\n",
		compOK, total, pct(compOK),
		defOK, total, pct(defOK),
		catOK, total, pct(catOK))
}

func renderCaseRow(w io.Writer, c caseResult) {
	comp := boolMark(c.ComponentCorrect)
	def := boolMark(c.DefectTypeCorrect)
	cat := boolMark(c.CategoryCorrect)

	job := c.Job
	if job == "" {
		job = "-"
	}

	conv := ""
	if c.ActualConvergence > 0 {
		conv = fmt.Sprintf("%.2f", c.ActualConvergence)
	}

	errStr := c.CircuitError
	if len(errStr) > 40 {
		errStr = errStr[:40] + "..."
	}

	ver := c.Version
	if len(ver) > 7 {
		ver = ver[:7]
	}
	if len(job) > 10 {
		job = job[:10]
	}

	fmt.Fprintf(w, "%-6s %-7s %-10s    %s      %s    %s  %5d  %4s  %s\n",
		c.CaseID, ver, job, comp, def, cat, c.StepCount, conv, errStr)
}

func boolMark(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}


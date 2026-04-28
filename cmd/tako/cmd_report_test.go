package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runReportCmd(t *testing.T, extraArgs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"--state-dir", testdataStateDir(t), "--run", "s-test-1"}, extraArgs...)
	if err := reportCmd(&buf, args); err != nil {
		t.Fatalf("reportCmd(%v) error: %v", args, err)
	}
	return buf.String()
}

func TestReportCmd_ScoreCard(t *testing.T) {
	out := runReportCmd(t)

	mustContain(t, out, "=== SCORECARD ===")
	mustContain(t, out, "M1")
	mustContain(t, out, "defect_type_accuracy")
	mustContain(t, out, "0.89")
	// M1 passed — should show "Y"
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, "M1") && strings.Contains(line, "defect_type_accuracy") {
			if !strings.Contains(line, "Y") {
				t.Errorf("M1 line should contain Y for passed, got: %s", line)
			}
			break
		}
	}
}

func TestReportCmd_ScoreCard_FailedMetric(t *testing.T) {
	out := runReportCmd(t)

	mustContain(t, out, "M2")
	mustContain(t, out, "0.77")
	// M2 failed — should show "N"
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, "M2") && strings.Contains(line, "category_accuracy") {
			if !strings.Contains(line, "N") {
				t.Errorf("M2 line should contain N for failed, got: %s", line)
			}
			break
		}
	}
}

func TestReportCmd_Cases(t *testing.T) {
	out := runReportCmd(t)

	mustContain(t, out, "C04")
	mustContain(t, out, "C05")
	mustContain(t, out, "C06")
	mustContain(t, out, "=== CASES (3) ===")
}

func TestReportCmd_Aggregate(t *testing.T) {
	out := runReportCmd(t)

	// Fixture: comp 1/3 (33%), defect 2/3 (66%), category 2/3 (66%)
	mustContain(t, out, "Component: 1/3 (33%)")
	mustContain(t, out, "Defect: 2/3 (66%)")
	mustContain(t, out, "Category: 2/3 (66%)")
}

func TestReportCmd_ScoreCardNotAllZero(t *testing.T) {
	// This test guards against the bug where scorecard showed "Passed: 0/0 Score: 0.00".
	// The golden fixture has Passed: 1/3 Score: 0.79.
	out := runReportCmd(t)

	mustContain(t, out, "Passed: 1/3")
	mustContain(t, out, "Score: 0.79")
	mustNotContain(t, out, "Passed: 0/0")
	mustNotContain(t, out, "Score: 0.00")
}

func TestReportCmd_JsonFormat(t *testing.T) {
	out := runReportCmd(t, "--format", "json")

	// Must be valid JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw:\n%s", err, out)
	}

	// Must contain expected keys.
	if _, ok := m["metrics"]; !ok {
		t.Error("JSON output missing 'metrics' key")
	}
	if _, ok := m["case_results"]; !ok {
		t.Error("JSON output missing 'case_results' key")
	}
}

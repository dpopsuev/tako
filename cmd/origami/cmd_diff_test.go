package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func runDiffCmd(t *testing.T, extraArgs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"--state-dir", testdataStateDir(t)}, extraArgs...)
	if err := diffCmd(&buf, args); err != nil {
		t.Fatalf("diffCmd(%v) error: %v", args, err)
	}
	return buf.String()
}

func TestDiffCmd_TextFormat(t *testing.T) {
	out := runDiffCmd(t, "s-test-1", "s-test-2")

	mustContain(t, out, "=== RUN DIFF ===")
	mustContain(t, out, "s-test-1")
	mustContain(t, out, "s-test-2")
	mustContain(t, out, "3 cases")
	mustContain(t, out, "4 cases")

	// M1: 0.89 -> 0.77 = -0.12 REGRESSED
	mustContain(t, out, "M1")
	mustContain(t, out, "defect_type_accuracy")
	mustContain(t, out, "-0.12")
	mustContain(t, out, "REGRESSED")

	// M2: 0.77 -> 0.85 = +0.08
	mustContain(t, out, "M2")
	mustContain(t, out, "+0.08")

	// M15: 0.72 -> 0.61 = -0.11 REGRESSED
	mustContain(t, out, "M15")
	mustContain(t, out, "-0.11")
}

func TestDiffCmd_JsonFormat(t *testing.T) {
	out := runDiffCmd(t, "--format", "json", "s-test-1", "s-test-2")

	var result diffOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw:\n%s", err, out)
	}

	if result.RunA.CaseCount != 3 {
		t.Errorf("RunA.CaseCount: got %d, want 3", result.RunA.CaseCount)
	}
	if result.RunB.CaseCount != 4 {
		t.Errorf("RunB.CaseCount: got %d, want 4", result.RunB.CaseCount)
	}
	if len(result.Metrics) != 3 {
		t.Errorf("Metrics count: got %d, want 3", len(result.Metrics))
	}

	// Check M1 delta.
	for _, m := range result.Metrics {
		if m.ID == "M1" {
			if m.ScoreA != 0.89 {
				t.Errorf("M1 ScoreA: got %f, want 0.89", m.ScoreA)
			}
			if m.ScoreB != 0.77 {
				t.Errorf("M1 ScoreB: got %f, want 0.77", m.ScoreB)
			}
			if !m.Regressed {
				t.Error("M1 should be marked as regressed")
			}
		}
	}
}

func TestDiffCmd_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := diffCmd(&buf, []string{"--state-dir", testdataStateDir(t), "s-test-1"})
	if err == nil {
		t.Fatal("expected error for missing second run argument")
	}
	mustContain(t, err.Error(), "usage")
}

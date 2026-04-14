package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testdataStateDir returns the absolute path to testdata/ which contains runs/s-test-1/.
func testdataStateDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func runTraceCmd(t *testing.T, extraArgs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"--state-dir", testdataStateDir(t), "--run", "s-test-1"}, extraArgs...)
	if err := traceCmd(&buf, args); err != nil {
		t.Fatalf("traceCmd(%v) error: %v", args, err)
	}
	return buf.String()
}

func TestTraceCmd_DefaultLevel(t *testing.T) {
	out := runTraceCmd(t)

	// Default level is info — only 4 of 9 lines are info-level.
	lines := nonEmptyLines(out)
	if len(lines) != 4 {
		t.Errorf("expected 4 info lines, got %d:\n%s", len(lines), out)
	}

	// All info events should be present.
	mustContain(t, out, "session_started")
	mustContain(t, out, "step_completed")

	// Debug/trace events must be absent.
	mustNotContain(t, out, "node_enter")
	mustNotContain(t, out, "edge_evaluate")
	mustNotContain(t, out, "node_exit")
}

func TestTraceCmd_VerboseLevel(t *testing.T) {
	out := runTraceCmd(t, "-v")

	// -v includes info + debug = 8 of 9 lines (excludes 1 trace-level).
	lines := nonEmptyLines(out)
	if len(lines) != 8 {
		t.Errorf("expected 8 info+debug lines, got %d:\n%s", len(lines), out)
	}

	mustContain(t, out, "node_enter")
	mustContain(t, out, "edge_evaluate")
	mustNotContain(t, out, "node_exit") // trace-level, not shown with -v
}

func TestTraceCmd_FilterByCase(t *testing.T) {
	out := runTraceCmd(t, "-v", "--case", "C04")

	// C04 has 5 events at info+debug level (lines 2,3,5,6,7 in fixture; line 4 is trace).
	for _, line := range nonEmptyLines(out) {
		if !strings.Contains(line, "C04") {
			t.Errorf("expected only C04 events, got line: %s", line)
		}
	}

	mustNotContain(t, out, "C05")
}

func TestTraceCmd_ErrorsOnly(t *testing.T) {
	// --errors shows only events with non-empty error field.
	// Need -v to include debug events, since the error event is debug-level.
	out := runTraceCmd(t, "-v", "--errors")

	lines := nonEmptyLines(out)
	if len(lines) != 1 {
		t.Errorf("expected 1 error line, got %d:\n%s", len(lines), out)
	}

	mustContain(t, out, "transformer failed")
}

func TestTraceCmd_JsonFormat(t *testing.T) {
	out := runTraceCmd(t, "--format", "json")

	// Default level is info, so 4 JSON lines.
	lines := nonEmptyLines(out)
	if len(lines) != 4 {
		t.Errorf("expected 4 JSON lines, got %d:\n%s", len(lines), out)
	}

	// Each line must be valid JSON.
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
	}
}

// --- TSK-189: --follow flag for delegation annotation ---

func TestTraceCmd_FollowAnnotatesDelegations(t *testing.T) {
	var buf bytes.Buffer
	args := []string{"--state-dir", testdataStateDir(t), "--run", "s-parent-1", "--follow"}
	if err := traceCmd(&buf, args); err != nil {
		t.Fatalf("traceCmd error: %v", err)
	}
	out := buf.String()

	// Should contain delegation markers.
	mustContain(t, out, "[DELEGATION START: beta]")
	mustContain(t, out, "[DELEGATION END: beta]")

	// Should inline child trace events (indented with circuit label).
	mustContain(t, out, "[beta]")
	mustContain(t, out, "tree")
	mustContain(t, out, "search")
}

func TestTraceCmd_FollowInlinesChildTrace(t *testing.T) {
	var buf bytes.Buffer
	args := []string{"--state-dir", testdataStateDir(t), "--run", "s-parent-1", "--follow", "--format", "json"}
	if err := traceCmd(&buf, args); err != nil {
		t.Fatalf("traceCmd error: %v", err)
	}
	out := buf.String()
	lines := nonEmptyLines(out)

	// Parent has 5 events; child has 3 inlined = 8 total.
	if len(lines) != 8 {
		t.Errorf("expected 8 JSON lines (5 parent + 3 child), got %d:\n%s", len(lines), out)
	}

	// Each line should be valid JSON.
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
	}
}

func TestTraceCmd_FollowWithoutDelegations(t *testing.T) {
	// --follow on a trace without delegation events should pass through unchanged.
	var buf bytes.Buffer
	args := []string{"--state-dir", testdataStateDir(t), "--run", "s-test-1", "--follow"}
	if err := traceCmd(&buf, args); err != nil {
		t.Fatalf("traceCmd error: %v", err)
	}
	out := buf.String()

	// Same 4 info-level lines as default (no delegation events).
	lines := nonEmptyLines(out)
	if len(lines) != 4 {
		t.Errorf("expected 4 info lines, got %d:\n%s", len(lines), out)
	}

	mustNotContain(t, out, "DELEGATION")
}

// --- helpers ---

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("output missing %q:\n%s", needle, haystack)
	}
}

func mustNotContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("output unexpectedly contains %q:\n%s", needle, haystack)
	}
}

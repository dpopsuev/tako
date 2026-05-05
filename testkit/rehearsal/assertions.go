package rehearsal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type Result struct {
	WorkDir  string
	Pass     bool
	Score    float64
	Events   []string
	Response string
}

func AssertPass(t *testing.T, r Result) {
	t.Helper()
	if !r.Pass {
		t.Fatalf("FAIL: score=%.2f", r.Score)
	}
	t.Logf("PASS: score=%.2f", r.Score)
}

func AssertUsedTools(t *testing.T, r Result, kinds ...string) {
	t.Helper()
	for _, kind := range kinds {
		found := false
		for _, e := range r.Events {
			if e == kind {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected tool event %q but it never fired", kind)
		}
	}
}

func AssertNotUsedTools(t *testing.T, r Result, kinds ...string) {
	t.Helper()
	for _, kind := range kinds {
		for _, e := range r.Events {
			if e == kind {
				t.Fatalf("tool event %q should not have fired", kind)
			}
		}
	}
}

func AssertFileExists(t *testing.T, r Result, relPath string) {
	t.Helper()
	full := filepath.Join(r.WorkDir, relPath)
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("expected file %s to exist: %v", relPath, err)
	}
}

func AssertCompiles(t *testing.T, r Result) {
	t.Helper()
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = r.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workspace does not compile:\n%s", out)
	}
}

func AssertTestsPass(t *testing.T, r Result) {
	t.Helper()
	cmd := exec.Command("go", "test", "./...", "-count=1")
	cmd.Dir = r.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tests fail in workspace:\n%s", out)
	}
}

func AssertRehearsal(t *testing.T, reh Rehearsal, r Result) {
	t.Helper()
	AssertPass(t, r)
	if len(reh.MustUse) > 0 {
		AssertUsedTools(t, r, reh.MustUse...)
	}
	if len(reh.MustNotUse) > 0 {
		AssertNotUsedTools(t, r, reh.MustNotUse...)
	}
	if reh.ExpectFile != "" {
		AssertFileExists(t, r, reh.ExpectFile)
	}
}

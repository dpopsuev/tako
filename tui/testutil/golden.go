// golden.go — Golden file comparison for TUI snapshot tests.
//
// Usage:
//
//	func TestMyWidget(t *testing.T) {
//	    w := NewMyWidget()
//	    testutil.RequireGolden(t, w.View(80))
//	}
//
// Run:    go test -tags golden ./tui/widgets/ -v
// Update: go test -tags golden -update ./tui/widgets/ -v
//
// Golden files live in testdata/ relative to the test package.
// ANSI is stripped before comparison — golden files are readable plaintext.
//
// GOL-188, TSK-1198
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// RequireGolden compares got against testdata/<testname>.golden.
// If -update is set, writes got to the golden file instead.
// ANSI is stripped from got before comparison.
func RequireGolden(tb testing.TB, got string) { //nolint:thelper // tb.Helper() called below
	tb.Helper()

	stripped := StripANSI(got)
	path := GoldenPath(tb)

	if *updateGolden {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
			tb.Fatalf("write golden file: %v", err)
		}
		tb.Logf("updated golden file: %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read golden file %s: %v\nRun with -update to create it.", path, err)
	}

	if stripped != string(want) {
		tb.Errorf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s",
			path, string(want), stripped)
	}
}

// GoldenPath returns the golden file path for the current test.
// Derived from t.Name(): slashes become underscores, stored in testdata/.
func GoldenPath(tb testing.TB) string { //nolint:thelper // tb.Helper() called below
	tb.Helper()
	name := strings.ReplaceAll(tb.Name(), "/", "_")
	return filepath.Join("testdata", name+".golden")
}

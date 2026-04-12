package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	if err := buildInstrumentBinaries(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build instruments: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// buildInstrumentBinaries finds all testkit/instruments/*/main.go and
// compiles each into a binary at testkit/instruments/<name>/<name>.
// Runs once per test package execution, not per test.
func buildInstrumentBinaries() error {
	root := repoRoot()
	pattern := filepath.Join(root, "testkit", "instruments", "*", "main.go")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, mainGo := range matches {
		dir := filepath.Dir(mainGo)
		name := filepath.Base(dir)
		bin := filepath.Join(dir, name)

		// Skip if binary is fresh (newer than source).
		if binInfo, err := os.Stat(bin); err == nil {
			srcInfo, _ := os.Stat(mainGo)
			if srcInfo != nil && !srcInfo.ModTime().After(binInfo.ModTime()) {
				continue
			}
		}

		pkg := "./testkit/instruments/" + name + "/"
		cmd := exec.Command("go", "build", "-o", bin, pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w\n%s", name, err, out)
		}
		fmt.Fprintf(os.Stderr, "built instrument: %s\n", name)
	}

	return nil
}

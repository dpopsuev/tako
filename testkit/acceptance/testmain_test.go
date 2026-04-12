package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// instrumentBinDir is the temp directory where compiled instrument binaries live.
// Set by TestMain, prepended to PATH so exec.LookPath finds them.
var instrumentBinDir string

func TestMain(m *testing.M) {
	var err error
	instrumentBinDir, err = os.MkdirTemp("", "origami-test-instruments-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(instrumentBinDir)

	if err := buildGoBinaries(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build instruments: %v\n", err)
		os.Exit(1)
	}

	// Prepend bin dir to PATH so exec.LookPath resolves instrument binaries.
	os.Setenv("PATH", instrumentBinDir+":"+os.Getenv("PATH"))

	os.Exit(m.Run())
}

// buildGoBinaries finds all testkit/instruments/*/main.go and compiles
// each into instrumentBinDir/<name>.
func buildGoBinaries() error {
	root := repoRoot()
	pattern := filepath.Join(root, "testkit", "instruments", "*", "main.go")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, mainGo := range matches {
		dir := filepath.Dir(mainGo)
		name := filepath.Base(dir)
		bin := filepath.Join(instrumentBinDir, name)

		pkg := "./testkit/instruments/" + name + "/"
		cmd := exec.Command("go", "build", "-o", bin, pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w\n%s", name, err, out)
		}
		fmt.Fprintf(os.Stderr, "built instrument: %s → %s\n", name, bin)
	}

	return nil
}

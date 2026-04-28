package operator

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func origamiRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("tako root not found at %s", root)
	}
	return root
}

func TestGitObserver_Observe_ReturnsCurrentState(t *testing.T) {
	root := origamiRoot(t)
	obs := NewGitObserver(root)

	state, err := obs.Observe()
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	if state.HeadSHA == "" {
		t.Error("HeadSHA is empty")
	}
	if len(state.HeadSHA) < 7 {
		t.Errorf("HeadSHA too short: %q", state.HeadSHA)
	}
	t.Logf("HEAD=%s findings=%d build=%v test=%v",
		state.HeadSHA[:7], state.ScanFindings, state.BuildPassing, state.TestPassing)
}

func TestGitObserver_SecondCall_SkipsScan(t *testing.T) {
	root := origamiRoot(t)
	obs := NewGitObserver(root)

	// First call — scans.
	state1, err := obs.Observe()
	if err != nil {
		t.Fatal(err)
	}

	// Second call — same SHA, should skip scan (faster).
	state2, err := obs.Observe()
	if err != nil {
		t.Fatal(err)
	}

	if state1.HeadSHA != state2.HeadSHA {
		t.Errorf("SHA changed between calls: %s vs %s", state1.HeadSHA, state2.HeadSHA)
	}
	// Second call should have 0 findings (scan skipped).
	if state2.ScanFindings != 0 {
		t.Errorf("expected 0 findings on second call (scan skipped), got %d", state2.ScanFindings)
	}
}

func TestGitObserver_Interface(t *testing.T) {
	var _ Observer = NewGitObserver(".")
}

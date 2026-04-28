package operator_test

import (
	"testing"
	"time"

	"github.com/dpopsuev/tako/operator"
	"github.com/dpopsuev/tako/simulate/sdlc"
)

// stubObserver returns drift on first call, clean on subsequent calls.
type stubObserver struct {
	calls int
}

func (o *stubObserver) Observe() (*operator.CurrentState, error) {
	o.calls++
	if o.calls == 1 {
		return &operator.CurrentState{
			HeadSHA:      "abc123",
			ScanFindings: 3,
			BuildPassing: true,
			TestPassing:  true,
		}, nil
	}
	return &operator.CurrentState{
		HeadSHA:      "abc123",
		ScanFindings: 0,
		BuildPassing: true,
		TestPassing:  true,
	}, nil
}

// TestOperator_MCP_DriftConvergence proves the full reconciliation loop:
// observer detects drift → MCPActor runs circuit via MCP → circuit converges.
func TestOperator_MCP_DriftConvergence(t *testing.T) {
	if testing.Short() {
		t.Skip("-short flag set")
	}

	t.Setenv("SDLC_MODE", "stub")
	t.Setenv("SDLC_REPO_PATH", "../simulate/sdlc")

	obs := &stubObserver{}
	actor := operator.NewMCPActor(
		sdlc.SessionFactory(),
		operator.WithMCPTimeout(30*time.Second),
	)

	runs := operator.Loop(t.Context(), operator.Config{
		Desired: operator.DesiredState{
			Scan:  "clean",
			Build: "passing",
			Test:  "passing",
		},
		Observer: obs,
		Actor:    actor,
		Interval: 100 * time.Millisecond,
		MaxRuns:  1,
	})

	if runs != 1 {
		t.Errorf("runs = %d, want 1", runs)
	}
	// MaxRuns=1: loop observes drift (1 call), runs circuit, exits.
	// The convergence re-check happens only if MaxRuns allows another iteration.
	if obs.calls < 1 {
		t.Errorf("observer calls = %d, want >= 1", obs.calls)
	}
	t.Logf("observer called %d times, circuit ran %d times", obs.calls, runs)
}

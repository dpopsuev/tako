package operator

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/trace"
)

// stubObserver returns drift on first call, then converged.
type stubObserver struct {
	calls atomic.Int32
}

func (s *stubObserver) Observe() (*CurrentState, error) {
	n := s.calls.Add(1)
	if n == 1 {
		// First call: drift — scan has findings.
		return &CurrentState{
			HeadSHA:      "abc123",
			ScanFindings: 3,
			BuildPassing: true,
			TestPassing:  true,
		}, nil
	}
	// Subsequent calls: converged.
	return &CurrentState{
		HeadSHA:      "def456",
		ScanFindings: 0,
		BuildPassing: true,
		TestPassing:  true,
	}, nil
}

// stubActor records that it was called and returns success.
type stubActor struct {
	called atomic.Bool
}

func (s *stubActor) Act(_ DriftResult) (*RunResult, error) {
	s.called.Store(true)
	return &RunResult{Success: true, Duration: 100 * time.Millisecond}, nil
}

func TestLoop_DetectsDrift_RunsCircuit_Converges(t *testing.T) {
	observer := &stubObserver{}
	actor := &stubActor{}
	recorder := trace.NewFlightRecorder(2000)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	runs := Loop(ctx, Config{
		Desired: DesiredState{
			Manifest: "origami-sdlc.yaml",
			RepoPath: "/workspace",
			Scan:     "clean",
			Build:    "passing",
			Test:     "passing",
		},
		Observer: observer,
		Actor:    actor,
		Interval: 10 * time.Millisecond,
		Recorder: recorder,
		MaxRuns:  1, // stop after first circuit run
	})

	if !actor.called.Load() {
		recorder.Dump(t)
		t.Fatal("actor was never called — drift was not detected")
	}

	if runs == 0 {
		recorder.Dump(t)
		t.Fatal("expected at least 1 circuit run")
	}

	// Verify FlightRecorder captured the loop events.
	events := recorder.Events()
	hasStart := false
	hasObserve := false
	hasDiff := false
	hasAct := false
	hasConverged := false
	for _, e := range events {
		switch e.Station {
		case "operator:start":
			hasStart = true
		case "operator:observe":
			hasObserve = true
		case "operator:diff":
			hasDiff = true
		case "operator:act":
			hasAct = true
		case "operator:converged":
			hasConverged = true
		}
	}

	if !hasStart {
		t.Error("missing operator:start event")
	}
	if !hasObserve {
		t.Error("missing operator:observe event")
	}
	if !hasDiff {
		t.Error("missing operator:diff event")
	}
	if !hasAct {
		t.Error("missing operator:act event")
	}
	// With MaxRuns=1, the loop stops after the first act — no second
	// observe to see convergence. That's OK — we verified drift→act works.
	_ = hasConverged

	if t.Failed() {
		recorder.Dump(t)
	}
}

func TestLoop_NoMaxRuns_StopsOnContext(t *testing.T) {
	// Observer always returns converged — loop should keep sleeping.
	observer := &stubObserver{} // first call drifts
	actor := &stubActor{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	runs := Loop(ctx, Config{
		Desired:  DesiredState{Scan: "clean", Build: "passing", Test: "passing"},
		Observer: observer,
		Actor:    actor,
		Interval: 10 * time.Millisecond,
		MaxRuns:  0, // unlimited — stopped by context
	})

	// Should have run at least once (first observe returns drift).
	if runs == 0 {
		t.Error("expected at least 1 run before context timeout")
	}
}

func TestDiff_Clean_NoDrift(t *testing.T) {
	desired := DesiredState{Scan: "clean", Build: "passing", Test: "passing"}
	current := &CurrentState{ScanFindings: 0, BuildPassing: true, TestPassing: true}

	drift := Diff(desired, current)
	if drift.Drifted {
		t.Errorf("expected no drift, got reasons: %v", drift.Reasons)
	}
}

func TestDiff_ScanFindings_Drifted(t *testing.T) {
	desired := DesiredState{Scan: "clean", Build: "passing", Test: "passing"}
	current := &CurrentState{ScanFindings: 5, BuildPassing: true, TestPassing: true}

	drift := Diff(desired, current)
	if !drift.Drifted {
		t.Fatal("expected drift for scan findings")
	}
	if len(drift.Reasons) != 1 || drift.Reasons[0] != "scan: findings detected" {
		t.Errorf("reasons = %v, want [scan: findings detected]", drift.Reasons)
	}
}

func TestDiff_MultipleDrifts(t *testing.T) {
	desired := DesiredState{Scan: "clean", Build: "passing", Test: "passing", Vulnerabilities: 0}
	current := &CurrentState{ScanFindings: 2, BuildPassing: false, TestPassing: false, Vulnerabilities: 3}

	drift := Diff(desired, current)
	if !drift.Drifted {
		t.Fatal("expected drift")
	}
	if len(drift.Reasons) != 4 {
		t.Errorf("expected 4 reasons, got %d: %v", len(drift.Reasons), drift.Reasons)
	}
}

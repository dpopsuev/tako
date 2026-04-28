package sdlc

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/engine/trace"
)

// Harness provides a fluent API for running SDLC circuit simulations.
// On failure, it auto-dumps the FlightRecorder timeline for diagnosis.
//
// Usage:
//
//	result := NewHarness(t).
//	    WithStubs(true).       // or WithInstruments(real)
//	    WithTimeout(10*time.Second).
//	    Run()
type Harness struct {
	t           testing.TB
	instruments engine.InstrumentRegistry
	recorder    *trace.FlightRecorder
	timeout     time.Duration
	parallel    int
}

// NewHarness creates a simulation harness for the given test.
func NewHarness(t testing.TB) *Harness { //nolint:thelper // NewHarness is a constructor, not a test helper
	return &Harness{
		t:        t,
		recorder: trace.NewFlightRecorder(1000),
		timeout:  10 * time.Second,
		parallel: 1,
	}
}

// WithStubs configures all-stub instruments. When clean is true, the scan
// stub returns no findings (clean path). When false, the first scan returns
// findings triggering the fix loop.
func (h *Harness) WithStubs(clean bool) *Harness {
	h.instruments = StubInstruments(clean)
	return h
}

// WithInstruments sets a custom instrument registry. Use this to mix
// real instruments with stubs — e.g., real Oculus scan + stub fix.
func (h *Harness) WithInstruments(reg engine.InstrumentRegistry) *Harness {
	h.instruments = reg
	return h
}

// WithInstrument replaces a single instrument in the registry.
// Panics if WithStubs or WithInstruments hasn't been called first.
func (h *Harness) WithInstrument(name string, tx engine.Instrument) *Harness {
	if h.instruments == nil {
		h.t.Fatal("WithInstrument called before WithStubs or WithInstruments")
	}
	h.instruments[name] = tx
	return h
}

// WithTimeout sets the run timeout. Default 10s.
func (h *Harness) WithTimeout(d time.Duration) *Harness {
	h.timeout = d
	return h
}

// WithParallel sets concurrent case execution. Default 1.
func (h *Harness) WithParallel(n int) *Harness {
	h.parallel = n
	return h
}

// WithRecorder uses a custom FlightRecorder. Default creates a new one.
func (h *Harness) WithRecorder(r *trace.FlightRecorder) *Harness {
	h.recorder = r
	return h
}

// Recorder returns the FlightRecorder for manual inspection.
func (h *Harness) Recorder() *trace.FlightRecorder {
	return h.recorder
}

// Run executes the SDLC circuit and returns the result. On error,
// auto-dumps the FlightRecorder timeline to the test log.
func (h *Harness) Run() *RunResult {
	h.t.Helper()
	if h.instruments == nil {
		h.t.Fatal("no instruments configured — call WithStubs() or WithInstruments()")
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Instruments: h.instruments,
		Recorder:    h.recorder,
		Parallel:    h.parallel,
	})
	if err != nil {
		h.recorder.Dump(h.t)
		h.t.Fatalf("SDLC simulation failed: %v", err)
	}

	// Check for walk errors.
	for _, wr := range result.WalkResults {
		if wr.Error != nil {
			h.recorder.Dump(h.t)
			h.t.Fatalf("SDLC walk error (case %s): %v", wr.CaseID, wr.Error)
		}
	}

	return result
}

// RunExpectError executes the SDLC circuit expecting a walk error.
// Returns the result for inspection. Does NOT auto-dump on error.
func (h *Harness) RunExpectError() *RunResult {
	h.t.Helper()
	if h.instruments == nil {
		h.t.Fatal("no instruments configured — call WithStubs() or WithInstruments()")
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Instruments: h.instruments,
		Recorder:    h.recorder,
		Parallel:    h.parallel,
	})
	if err != nil {
		h.t.Logf("SDLC simulation error (expected): %v", err)
		return nil
	}
	return result
}

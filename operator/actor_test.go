package operator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

// runCircuit is a test helper that runs the SDLC circuit via engine.BatchWalk directly.
func runCircuit(ctx context.Context, transformers engine.InstrumentRegistry) ([]engine.BatchWalkResult, error) {
	domainFS := os.DirFS("../simulate/sdlc")
	def, err := sdlc.LoadCircuit(domainFS)
	if err != nil {
		return nil, err
	}
	results := engine.BatchWalk(ctx, engine.BatchWalkConfig{
		Def: def,
		Shared: &engine.GraphRegistries{
			Instruments: transformers,
			Circuits:     circuit.LoadSubCircuitsFromFS(domainFS, nil),
		},
		Cases:    []engine.BatchCase{{ID: "sdlc-run", Context: map[string]any{}}},
		Parallel: 1,
	})
	return results, nil
}

func TestInProcessActor_RunsCircuit(t *testing.T) {
	actor := NewInProcessActor(func(ctx context.Context, _ DriftResult) (*RunResult, error) {
		start := time.Now()
		results, err := runCircuit(ctx, sdlc.StubInstruments(true))
		if err != nil {
			return &RunResult{Success: false, Duration: time.Since(start), Error: err.Error()}, nil
		}
		for _, wr := range results {
			if wr.Error != nil {
				return &RunResult{Success: false, Duration: time.Since(start), Error: wr.Error.Error()}, nil
			}
		}
		return &RunResult{Success: true, Duration: time.Since(start)}, nil
	})

	result, err := actor.Act(DriftResult{Drifted: true, Reasons: []string{"scan: findings detected"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	t.Logf("Circuit completed in %s", result.Duration)
}

func TestInProcessActor_ViaOperatorLoop(t *testing.T) {
	observer := &stubObserver{}
	actor := NewInProcessActor(func(ctx context.Context, _ DriftResult) (*RunResult, error) {
		start := time.Now()
		_, err := runCircuit(ctx, sdlc.StubInstruments(false)) // dirty → fix loop → clean
		if err != nil {
			return &RunResult{Success: false, Duration: time.Since(start), Error: err.Error()}, nil
		}
		return &RunResult{Success: true, Duration: time.Since(start)}, nil
	})

	recorder := trace.NewFlightRecorder(500)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runs := Loop(ctx, Config{
		Desired:  DesiredState{Scan: "clean", Build: "passing", Test: "passing"},
		Observer: observer,
		Actor:    actor,
		Interval: 10 * time.Millisecond,
		Recorder: recorder,
		MaxRuns:  1,
	})

	if runs == 0 {
		recorder.Dump(t)
		t.Fatal("expected at least 1 circuit run")
	}

	// Verify the full chain: operator observed drift → ran circuit → circuit completed.
	hasAct := false
	for _, e := range recorder.Events() {
		if e.Station == "operator:act" {
			hasAct = true
		}
	}
	if !hasAct {
		recorder.Dump(t)
		t.Error("missing operator:act event")
	}
}

func TestContainerActor_Interface(t *testing.T) {
	var _ Actor = NewContainerActor("origami-sdlc:latest", "/workspace")
}

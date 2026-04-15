package sdlc

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

func TestSDLCCircuit_Validates(t *testing.T) {
	def, err := LoadCircuit(os.DirFS("."))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestSDLCCircuit_FinallyNode(t *testing.T) {
	def, err := LoadCircuit(os.DirFS("."))
	if err != nil {
		t.Fatal(err)
	}
	if def.Finally == "" {
		t.Fatal("finally node not set")
	}
	if string(def.Finally) != "teardown" {
		t.Errorf("finally = %q, want teardown", def.Finally)
	}
	// Verify teardown exists in node list.
	found := false
	for _, n := range def.Nodes {
		if string(n.Name) == "teardown" {
			found = true
			break
		}
	}
	if !found {
		t.Error("teardown node not found in nodes list")
	}
}

func TestSDLCTypes_AllOutputsTyped(t *testing.T) {
	// Every stub transformer must return a pointer to a typed struct,
	// not map[string]any. This is the hard-typing contract.
	stubs := StubInstruments(true)
	ctx := context.Background()

	for name, tx := range stubs {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.InstrumentContext{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			// Must be a pointer to a struct, not a map.
			switch result.(type) {
			case *sdlctype.ScanResult, *sdlctype.FixResult, *sdlctype.BuildResult, *sdlctype.TestResult,
				*sdlctype.SelfReviewResult, *sdlctype.DeployResult, *sdlctype.ValidateResult,
				*sdlctype.HardenResult, *sdlctype.ReleaseResult, *sdlctype.TeardownResult,
				// V2 sub-circuit types
				*sdlctype.PollScribeResult, *sdlctype.ResolveContextResult, *sdlctype.GateResult,
				*sdlctype.CreateWorktreeResult, *sdlctype.WriteTestResult, *sdlctype.WriteCodeResult,
				*sdlctype.RefactorResult, *sdlctype.LintResult, *sdlctype.SecurityScanResult,
				*sdlctype.MonitorHealthResult, *sdlctype.PromoteResult, *sdlctype.RollbackResult,
				*sdlctype.FileBugResult, *sdlctype.MarkDoneResult:
				// OK — typed struct
			default:
				t.Errorf("expected typed struct, got %T", result)
			}
		})
	}
}

func TestSDLC_FullPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	recorder := trace.NewFlightRecorder(1000)
	result, err := Run(ctx, RunConfig{
		Instruments: StubInstruments(true),
		Recorder:     recorder,
	})
	if err != nil {
		recorder.Dump(t)
		t.Fatalf("Run: %v", err)
	}
	if len(result.WalkResults) != 1 {
		t.Fatalf("expected 1 walk result, got %d", len(result.WalkResults))
	}
	wr := result.WalkResults[0]
	if wr.Error != nil {
		recorder.Dump(t)
		t.Fatalf("walk error: %v", wr.Error)
	}

	// Fan-Out/Fan-In pipeline: plan → code → verify → publish → teardown.
	// All delegate nodes + teardown should have artifacts.
	for _, node := range []string{"plan", "code", "verify", "publish", "teardown"} {
		if _, ok := wr.StepArtifacts[node]; !ok {
			recorder.Dump(t)
			t.Errorf("missing artifact for delegate node %q", node)
		}
	}

	t.Logf("pipeline: %d artifacts", len(wr.StepArtifacts))
}

func TestSDLC_FlightRecorderCaptures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recorder := trace.NewFlightRecorder(1000)
	_, err := Run(ctx, RunConfig{
		Instruments: StubInstruments(true),
		Recorder:     recorder,
	})
	if err != nil {
		recorder.Dump(t)
		t.Fatal(err)
	}

	events := recorder.Events()
	if len(events) < 10 {
		recorder.Dump(t)
		t.Fatalf("expected at least 10 events (v2 pipeline), got %d", len(events))
	}

	// Should have session:start and circuit:complete bookends.
	hasStart := false
	hasComplete := false
	for _, e := range events {
		if e.Station == "session:start" {
			hasStart = true
		}
		if e.Station == "circuit:complete" {
			hasComplete = true
		}
	}
	if !hasStart {
		t.Error("missing session:start event")
	}
	if !hasComplete {
		t.Error("missing circuit:complete event")
	}
}

func TestSDLCV2_SubCircuitDelegation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	domainFS := os.DirFS(".")

	// Load v2 parent circuit.
	sdlcDef, err := LoadCircuit(domainFS)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	// Load sub-circuits from filesystem.
	subCircuits := circuit.LoadSubCircuitsFromFS(domainFS, nil)
	if subCircuits == nil {
		t.Fatal("no sub-circuits loaded from circuits/ directory")
	}

	// Verify all 4 sub-circuits loaded.
	for _, name := range []string{"planning", "coding", "verifying", "publishing"} {
		if _, ok := subCircuits[name]; !ok {
			t.Fatalf("sub-circuit %q not loaded", name)
		}
	}

	// Build registries with all stubs + sub-circuits.
	shared := &engine.GraphRegistries{
		Instruments: StubInstruments(true),
		Circuits:     subCircuits,
	}

	cases := []engine.BatchCase{
		{ID: "v2-e2e", Context: map[string]any{}},
	}

	results := engine.BatchWalk(ctx, engine.BatchWalkConfig{
		Def:      sdlcDef,
		Shared:   shared,
		Cases:    cases,
		Parallel: 1,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 walk result, got %d", len(results))
	}

	wr := results[0]
	if wr.Error != nil {
		t.Fatalf("v2 walk error: %v", wr.Error)
	}

	// All 4 delegate nodes should have artifacts.
	for _, node := range []string{"plan", "code", "verify", "publish"} {
		if _, ok := wr.StepArtifacts[node]; !ok {
			t.Errorf("missing artifact for delegate node %q", node)
		}
	}

	// Teardown (finally) should have run.
	if _, ok := wr.StepArtifacts["teardown"]; !ok {
		t.Error("missing teardown artifact (finally node)")
	}

	t.Logf("v2 E2E: %d nodes walked, %d artifacts", len(wr.StepArtifacts), len(wr.StepArtifacts))
}

package sdlc

import (
	"context"
	"os"
	"testing"
	"time"

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
	stubs := StubTransformers(true)
	ctx := context.Background()

	for name, tx := range stubs {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.TransformerContext{})
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

func TestSDLC_CleanPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recorder := trace.NewFlightRecorder(1000)
	result, err := Run(ctx, RunConfig{
		Transformers: StubTransformers(true), // clean scan — no findings
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

	// Clean path: scan → harden → release → done.
	// Teardown runs via finally.
	expectedNodes := []string{"scan", "harden", "release", "teardown"}
	for _, node := range expectedNodes {
		if _, ok := wr.StepArtifacts[node]; !ok {
			recorder.Dump(t)
			t.Errorf("missing artifact for node %q", node)
		}
	}
	// Fix loop nodes should NOT appear.
	for _, skip := range []string{"fix", "build", "test", "deploy-canary", "validate"} {
		if _, ok := wr.StepArtifacts[skip]; ok {
			t.Errorf("unexpected artifact for node %q (should be skipped on clean path)", skip)
		}
	}
}

func TestSDLC_FixLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recorder := trace.NewFlightRecorder(1000)
	result, err := Run(ctx, RunConfig{
		Transformers: StubTransformers(false), // dirty scan — findings on first call
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

	// Fix loop path: scan(dirty)→fix→build→test→deploy→validate→scan(clean)→harden→release→done.
	// All nodes should have artifacts.
	for _, node := range []string{"scan", "fix", "build", "test", "deploy-canary", "validate", "harden", "release", "teardown"} {
		if _, ok := wr.StepArtifacts[node]; !ok {
			recorder.Dump(t)
			t.Errorf("missing artifact for node %q", node)
		}
	}
}

func TestSDLC_FlightRecorderCaptures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recorder := trace.NewFlightRecorder(1000)
	_, err := Run(ctx, RunConfig{
		Transformers: StubTransformers(true),
		Recorder:     recorder,
	})
	if err != nil {
		recorder.Dump(t)
		t.Fatal(err)
	}

	events := recorder.Events()
	if len(events) < 5 {
		recorder.Dump(t)
		t.Fatalf("expected at least 5 events, got %d", len(events))
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

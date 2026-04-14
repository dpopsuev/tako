package sdlc

import (
	"testing"

	"github.com/dpopsuev/origami/engine/trace"
)

func TestHarness_CleanPath(t *testing.T) {
	result := NewHarness(t).
		WithStubs(true).
		Run()

	if len(result.WalkResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.WalkResults))
	}
	// V2 pipeline: plan → code → verify → ship → teardown.
	for _, node := range []string{"plan", "code", "verify", "ship", "teardown"} {
		if _, ok := result.WalkResults[0].StepArtifacts[node]; !ok {
			t.Errorf("missing artifact for %s", node)
		}
	}
}

func TestHarness_FixLoop(t *testing.T) {
	result := NewHarness(t).
		WithStubs(false).
		Run()

	// V2 pipeline walks same nodes regardless of clean/dirty (stubs always pass).
	for _, node := range []string{"plan", "code", "verify", "ship", "teardown"} {
		if _, ok := result.WalkResults[0].StepArtifacts[node]; !ok {
			t.Errorf("missing artifact for %s", node)
		}
	}
}

func TestHarness_WithRecorder_CustomRecorder(t *testing.T) {
	recorder := trace.NewFlightRecorder(50)

	result := NewHarness(t).
		WithStubs(true).
		WithRecorder(recorder).
		Run()

	if result.Recorder != recorder {
		t.Error("expected custom recorder in result")
	}
	events := recorder.Events()
	if len(events) == 0 {
		t.Error("custom recorder has no events")
	}
}

func TestHarness_Recorder_Accessible(t *testing.T) {
	h := NewHarness(t).WithStubs(true)
	recorder := h.Recorder()
	if recorder == nil {
		t.Fatal("Recorder() returned nil before Run()")
	}
}

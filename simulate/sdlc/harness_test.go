package sdlc

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
)

func TestHarness_CleanPath(t *testing.T) {
	result := NewHarness(t).
		WithStubs(true).
		Run()

	if len(result.WalkResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.WalkResults))
	}
	// Clean path should visit scan, harden, release, teardown.
	for _, node := range []string{"scan", "harden", "release", "teardown"} {
		if _, ok := result.WalkResults[0].StepArtifacts[node]; !ok {
			t.Errorf("missing artifact for %s", node)
		}
	}
}

func TestHarness_FixLoop(t *testing.T) {
	result := NewHarness(t).
		WithStubs(false).
		Run()

	// Fix loop should visit all nodes.
	for _, node := range []string{"scan", "fix", "build", "test", "deploy-canary", "validate", "harden", "release", "teardown"} {
		if _, ok := result.WalkResults[0].StepArtifacts[node]; !ok {
			t.Errorf("missing artifact for %s", node)
		}
	}
}

func TestHarness_WithTransformer_SwapsOne(t *testing.T) {
	var customCalled bool
	custom := engine.TransformerFunc("harden", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		customCalled = true
		return &HardenResult{Vulnerabilities: 3, PinnedDeps: []string{"custom-dep"}}, nil
	})

	result := NewHarness(t).
		WithStubs(true).
		WithTransformer("harden", custom).
		Run()

	if !customCalled {
		t.Error("custom harden transformer was not called")
	}
	art := result.WalkResults[0].StepArtifacts["harden"]
	if art == nil {
		t.Fatal("harden artifact is nil")
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

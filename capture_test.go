package framework

import (
	"sync"
	"testing"
)

type captureTestArtifact string

func (a captureTestArtifact) Type() string       { return "test" }
func (a captureTestArtifact) Confidence() float64 { return 1.0 }
func (a captureTestArtifact) Raw() any            { return string(a) }

func TestOutputCapture_CapturesNodeExitArtifacts(t *testing.T) {
	capture := newOutputCapture()

	capture.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "recall"})
	capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: "recall", Artifact: captureTestArtifact("data-1")})
	capture.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "triage"})
	capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: "triage", Artifact: captureTestArtifact("data-2")})
	capture.OnEvent(WalkEvent{Type: EventWalkComplete})

	arts := capture.Artifacts()
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	if _, ok := arts["recall"]; !ok {
		t.Fatal("missing recall artifact")
	}
	if _, ok := arts["triage"]; !ok {
		t.Fatal("missing triage artifact")
	}
}

func TestOutputCapture_IgnoresNilArtifacts(t *testing.T) {
	capture := newOutputCapture()
	capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: "empty"})

	arts := capture.Artifacts()
	if len(arts) != 0 {
		t.Fatalf("expected 0 artifacts, got %d", len(arts))
	}
}

func TestOutputCapture_ArtifactAt(t *testing.T) {
	capture := newOutputCapture()
	capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: "report", Artifact: captureTestArtifact("result")})

	art, ok := capture.ArtifactAt("report")
	if !ok {
		t.Fatal("expected artifact at 'report'")
	}
	if art == nil {
		t.Fatal("artifact should not be nil")
	}

	_, ok = capture.ArtifactAt("missing")
	if ok {
		t.Fatal("should not find 'missing'")
	}
}

func TestOutputCapture_ConcurrentSafety(t *testing.T) {
	capture := newOutputCapture()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			node := "node"
			capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: node, Artifact: captureTestArtifact("data")})
			capture.Artifacts()
			capture.ArtifactAt(node)
		}(i)
	}
	wg.Wait()
}

func TestOutputCapture_Reset(t *testing.T) {
	capture := newOutputCapture()
	capture.OnEvent(WalkEvent{Type: EventNodeExit, Node: "a", Artifact: captureTestArtifact("x")})
	capture.Reset()

	if len(capture.Artifacts()) != 0 {
		t.Fatal("expected empty after reset")
	}
}

// Tests for withOutputCapture composition moved to engine/ (internal runConfig).

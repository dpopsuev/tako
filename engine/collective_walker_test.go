package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

func TestCollectiveWalker_LinearWithTwoWalkers(t *testing.T) {
	nodeA := &stubNode{name: "classify", element: roster.ElementFire, artifact: &stubArtifact{typ: "classification", confidence: 0.9}}
	nodeB := &stubNode{name: "investigate", element: roster.ElementWater, artifact: &stubArtifact{typ: "investigation", confidence: 0.8}}
	nodeC := &stubNode{name: "decide", element: roster.ElementEarth, artifact: &stubArtifact{typ: "decision", confidence: 0.95}}

	edges := []circuit.Edge{
		&stubEdge{id: "E1", from: "classify", to: "investigate"},
		&stubEdge{id: "E2", from: "investigate", to: "decide"},
		&stubEdge{id: "E3", from: "decide", to: "_done"},
	}

	g, err := NewGraph("triage", []circuit.Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	herald := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "Herald",
			Element:      roster.ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.1, "decide": 0.5},
		},
		state: circuit.NewWalkerState("herald-1"),
	}
	seeker := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "Seeker",
			Element:      roster.ElementWater,
			StepAffinity: map[string]float64{"classify": 0.1, "investigate": 0.9, "decide": 0.3},
		},
		state: circuit.NewWalkerState("seeker-1"),
	}

	tc := &TraceCollector{}
	cw := NewCollectiveWalker(
		[]circuit.Walker{herald, seeker},
		&AffinitySelector{},
		WithCollectiveObserver(tc),
	)

	if err := g.Walk(context.Background(), cw, "classify"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	switches := tc.EventsOfType(circuit.EventWalkerSwitch)
	if len(switches) < 2 {
		t.Errorf("expected at least 2 walker switches, got %d", len(switches))
	}
}

func TestCollectiveWalker_MaxStepsGuard(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []circuit.Edge{
		&stubEdge{id: "A-loop", from: "A", to: "A"},
	}

	g, err := NewGraph("loop", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: roster.AgentIdentity{Name: "Solo"},
		state:    circuit.NewWalkerState("solo-1"),
	}

	cw := NewCollectiveWalker(
		[]circuit.Walker{w},
		&AffinitySelector{},
		WithMaxSteps(3),
	)

	err = g.Walk(context.Background(), cw, "A")
	if err == nil {
		t.Fatal("expected max steps error")
	}
	if !errors.Is(err, ErrMaxStepsExceeded) {
		t.Errorf("expected ErrMaxStepsExceeded, got %v", err)
	}
}

func TestCollectiveWalker_NilObserver(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []circuit.Edge{
		&stubEdge{id: "A-done", from: "A", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: roster.AgentIdentity{Name: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}

	cw := NewCollectiveWalker([]circuit.Walker{w}, &AffinitySelector{})

	if err := g.Walk(context.Background(), cw, "A"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectiveWalker_MismatchEmitted(t *testing.T) {
	nodeA := &stubNode{name: "A", element: roster.ElementFire, artifact: &stubArtifact{typ: "a", confidence: 0.8}}
	nodeB := &stubNode{name: "B", element: roster.ElementWater, artifact: &stubArtifact{typ: "b", confidence: 0.7}}

	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("mismatch-test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	wFire := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "FireWalker",
			Element:      roster.ElementFire,
			StepAffinity: map[string]float64{"A": 0.9, "B": 0.1},
		},
		state: circuit.NewWalkerState("fire-1"),
	}
	wWater := &stubWalker{
		identity: roster.AgentIdentity{
			Name:         "WaterWalker",
			Element:      roster.ElementWater,
			StepAffinity: map[string]float64{"A": 0.1, "B": 0.9},
		},
		state: circuit.NewWalkerState("water-1"),
	}

	tc := &TraceCollector{}
	cw := NewCollectiveWalker(
		[]circuit.Walker{wFire, wWater},
		&AffinitySelector{},
		WithCollectiveObserver(tc),
		WithMaxSteps(10),
	)

	if err := g.Walk(context.Background(), cw, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	switches := tc.EventsOfType(circuit.EventWalkerSwitch)
	if len(switches) < 2 {
		t.Fatalf("expected at least 2 walker_switch events, got %d", len(switches))
	}

	foundMismatch := false
	for _, ev := range switches {
		if ev.Metadata == nil {
			t.Error("walker_switch event has nil Metadata, expected mismatch key")
			continue
		}
		val, ok := ev.Metadata["mismatch"]
		if !ok {
			t.Errorf("walker_switch event for %s missing mismatch key", ev.Walker)
			continue
		}
		mm, ok := val.(float64)
		if !ok {
			t.Errorf("mismatch value is %T, want float64", val)
			continue
		}
		if mm > 0 {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Error("expected at least one walker_switch with mismatch > 0")
	}
}

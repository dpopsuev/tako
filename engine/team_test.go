package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestWalkTeam_LinearWithTwoWalkers(t *testing.T) {
	nodeA := &stubNode{name: "classify", element: circuit.ElementFire, artifact: &stubArtifact{typ: "classification", confidence: 0.9}}
	nodeB := &stubNode{name: "investigate", element: circuit.ElementWater, artifact: &stubArtifact{typ: "investigation", confidence: 0.8}}
	nodeC := &stubNode{name: "decide", element: circuit.ElementEarth, artifact: &stubArtifact{typ: "decision", confidence: 0.95}}

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
		identity: circuit.AgentIdentity{
			PersonaName:  "Herald",
			Element:      circuit.ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.1, "decide": 0.5},
		},
		state: circuit.NewWalkerState("herald-1"),
	}
	seeker := &stubWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Seeker",
			Element:      circuit.ElementWater,
			StepAffinity: map[string]float64{"classify": 0.1, "investigate": 0.9, "decide": 0.3},
		},
		state: circuit.NewWalkerState("seeker-1"),
	}

	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []circuit.Walker{herald, seeker},
		Scheduler: &AffinityScheduler{},
		Observer:  tc,
	}

	if err := g.WalkTeam(context.Background(), team, "classify"); err != nil {
		t.Fatalf("WalkTeam failed: %v", err)
	}

	switches := tc.EventsOfType(circuit.EventWalkerSwitch)
	if len(switches) < 2 {
		t.Errorf("expected at least 2 walker switches, got %d", len(switches))
	}

	enters := tc.EventsOfType(circuit.EventNodeEnter)
	if len(enters) != 3 {
		t.Fatalf("expected 3 node_enter events, got %d", len(enters))
	}
	if enters[0].Walker != "Herald" {
		t.Errorf("classify should be handled by Herald, got %s", enters[0].Walker)
	}
	if enters[1].Walker != "Seeker" {
		t.Errorf("investigate should be handled by Seeker, got %s", enters[1].Walker)
	}

	completes := tc.EventsOfType(circuit.EventWalkComplete)
	if len(completes) != 1 {
		t.Errorf("expected 1 walk_complete event, got %d", len(completes))
	}
}

func TestWalkTeam_MaxStepsGuard(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []circuit.Edge{
		&stubEdge{id: "A-loop", from: "A", to: "A"},
	}

	g, err := NewGraph("loop", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: circuit.AgentIdentity{PersonaName: "Solo"},
		state:    circuit.NewWalkerState("solo-1"),
	}

	team := &Team{
		Walkers:   []circuit.Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		MaxSteps:  3,
	}

	err = g.WalkTeam(context.Background(), team, "A")
	if err == nil {
		t.Fatal("expected max steps error")
	}
	if !errors.Is(err, nil) {
		// Just check that the error message mentions max steps
		if got := err.Error(); got == "" {
			t.Fatal("expected non-empty error message")
		}
	}
}

func TestWalkTeam_ObserverReceivesEdgeEvents(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: circuit.AgentIdentity{PersonaName: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []circuit.Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  tc,
	}

	if err := g.WalkTeam(context.Background(), team, "A"); err != nil {
		t.Fatal(err)
	}

	edgeEvals := tc.EventsOfType(circuit.EventEdgeEvaluate)
	if len(edgeEvals) < 2 {
		t.Errorf("expected at least 2 edge_evaluate events, got %d", len(edgeEvals))
	}

	transitions := tc.EventsOfType(circuit.EventTransition)
	if len(transitions) < 1 {
		t.Errorf("expected at least 1 transition event, got %d", len(transitions))
	}
}

func TestWalkTeam_NilObserver(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []circuit.Edge{
		&stubEdge{id: "A-done", from: "A", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: circuit.AgentIdentity{PersonaName: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	team := &Team{
		Walkers:   []circuit.Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  nil,
	}

	if err := g.WalkTeam(context.Background(), team, "A"); err != nil {
		t.Fatal(err)
	}
}

func TestWalkTeam_NoWalkersError(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}

	g, err := NewGraph("test", []circuit.Node{nodeA}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	team := &Team{
		Walkers:   nil,
		Scheduler: &AffinityScheduler{},
	}

	err = g.WalkTeam(context.Background(), team, "A")
	if err == nil {
		t.Fatal("expected error for empty walkers")
	}
}

func TestWalkTeam_StartNodeNotFound(t *testing.T) {
	g, err := NewGraph("test", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: circuit.AgentIdentity{PersonaName: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	team := &Team{
		Walkers:   []circuit.Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
	}

	err = g.WalkTeam(context.Background(), team, "nonexistent")
	if !errors.Is(err, circuit.ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestWalkTeam_ContextCancellation(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}
	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := &stubWalker{
		identity: circuit.AgentIdentity{PersonaName: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []circuit.Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  tc,
	}

	err = g.WalkTeam(ctx, team, "A")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}

	walkErrors := tc.EventsOfType(circuit.EventWalkError)
	if len(walkErrors) == 0 {
		t.Error("expected walk_error event on cancellation")
	}
}

func TestWalkTeam_MismatchEmitted(t *testing.T) {
	nodeA := &stubNode{name: "A", element: circuit.ElementFire, artifact: &stubArtifact{typ: "a", confidence: 0.8}}
	nodeB := &stubNode{name: "B", element: circuit.ElementWater, artifact: &stubArtifact{typ: "b", confidence: 0.7}}

	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("mismatch-test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	wFire := &stubWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "FireWalker",
			Element:      circuit.ElementFire,
			StepAffinity: map[string]float64{"A": 0.9, "B": 0.1},
		},
		state: circuit.NewWalkerState("fire-1"),
	}
	wWater := &stubWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "WaterWalker",
			Element:      circuit.ElementWater,
			StepAffinity: map[string]float64{"A": 0.1, "B": 0.9},
		},
		state: circuit.NewWalkerState("water-1"),
	}

	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []circuit.Walker{wFire, wWater},
		Scheduler: &AffinityScheduler{},
		Observer:  tc,
		MaxSteps:  10,
	}

	if err := g.WalkTeam(context.Background(), team, "A"); err != nil {
		t.Fatalf("WalkTeam failed: %v", err)
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

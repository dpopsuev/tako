package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// --- Local test helpers specific to graph_test.go ---

// visitTrackingWalker wraps stubWalker to track visited nodes
type visitTrackingWalker struct {
	identity circuit.AgentIdentity
	state    *circuit.WalkerState
	visited  []string
}

func (w *visitTrackingWalker) Identity() circuit.AgentIdentity       { return w.identity }
func (w *visitTrackingWalker) SetIdentity(id *circuit.AgentIdentity) { w.identity = *id }
func (w *visitTrackingWalker) State() *circuit.WalkerState        { return w.state }
func (w *visitTrackingWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	w.visited = append(w.visited, node.Name())
	return node.Process(ctx, nc)
}

type stubEdgeWithEvalFn struct {
	id, from, to string
	shortcut     bool
	loop         bool
	parallel     bool
	evalFn       func(circuit.Artifact, *circuit.WalkerState) *circuit.Transition
}

func (e *stubEdgeWithEvalFn) ID() string       { return e.id }
func (e *stubEdgeWithEvalFn) From() string     { return e.from }
func (e *stubEdgeWithEvalFn) To() string       { return e.to }
func (e *stubEdgeWithEvalFn) IsShortcut() bool { return e.shortcut }
func (e *stubEdgeWithEvalFn) IsLoop() bool     { return e.loop }
func (e *stubEdgeWithEvalFn) IsParallel() bool { return e.parallel }
func (e *stubEdgeWithEvalFn) Evaluate(a circuit.Artifact, s *circuit.WalkerState) *circuit.Transition {
	if e.evalFn != nil {
		return e.evalFn(a, s)
	}
	return &circuit.Transition{NextNode: e.to, Explanation: e.id + " matched"}
}

type countableStubArtifact struct {
	typ        string
	confidence float64
	inputN     int
	outputN    int
}

func (a *countableStubArtifact) Type() string        { return a.typ }
func (a *countableStubArtifact) Confidence() float64 { return a.confidence }
func (a *countableStubArtifact) Raw() any            { return nil }
func (a *countableStubArtifact) InputCount() int     { return a.inputN }
func (a *countableStubArtifact) OutputCount() int    { return a.outputN }

var _ circuit.CountableArtifact = (*countableStubArtifact)(nil)

// --- tests ---

func TestGraph_LinearWalk(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b", confidence: 1.0}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []circuit.Edge{
		&stubEdge{id: "E1", from: "A", to: "B"},
		&stubEdge{id: "E2", from: "B", to: "C"},
		&stubEdge{id: "E3", from: "C", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-1")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if w.state.Status != "done" {
		t.Errorf("expected status done, got %s", w.state.Status)
	}
	if len(w.visited) != 3 {
		t.Errorf("expected 3 visited nodes, got %d: %v", len(w.visited), w.visited)
	}
	want := []string{"A", "B", "C"}
	for i, v := range want {
		if w.visited[i] != v {
			t.Errorf("visited[%d] = %s, want %s", i, w.visited[i], v)
		}
	}
	if len(w.state.History) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(w.state.History))
	}
}

func TestGraph_ShortcutEdge(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 0.95}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b", confidence: 1.0}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []circuit.Edge{
		&stubEdgeWithEvalFn{
			id: "shortcut", from: "A", to: "C", shortcut: true,
			evalFn: func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
				if a.Confidence() >= 0.9 {
					return &circuit.Transition{NextNode: "C", Explanation: "high confidence shortcut"}
				}
				return nil
			},
		},
		&stubEdge{id: "normal", from: "A", to: "B"},
		&stubEdge{id: "B-to-C", from: "B", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-2")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(w.visited) != 2 {
		t.Errorf("expected 2 visited nodes (A, C), got %d: %v", len(w.visited), w.visited)
	}
	if w.visited[0] != "A" || w.visited[1] != "C" {
		t.Errorf("expected [A, C], got %v", w.visited)
	}
}

func TestGraph_LoopEdge(t *testing.T) {
	callCount := 0
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeB := &stubNode{
		name:     "B",
		artifact: &stubArtifact{typ: "b", confidence: 0.5},
	}

	maxLoops := 2
	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdgeWithEvalFn{
			id: "B-loop", from: "B", to: "B", loop: true,
			evalFn: func(a circuit.Artifact, s *circuit.WalkerState) *circuit.Transition {
				callCount++
				if s.LoopCounts["B-loop"] < maxLoops {
					s.IncrementLoop("B-loop")
					return &circuit.Transition{NextNode: "B", Explanation: "loop again"}
				}
				return nil
			},
		},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-3")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// A visited once, B visited 1 (initial) + 2 (loops) = 3
	if len(w.visited) != 4 {
		t.Errorf("expected 4 visits (A + B*3), got %d: %v", len(w.visited), w.visited)
	}
	if w.state.LoopCounts["B-loop"] != maxLoops {
		t.Errorf("expected loop count %d, got %d", maxLoops, w.state.LoopCounts["B-loop"])
	}
}

func TestGraph_ErrNodeNotFound_StartNode(t *testing.T) {
	g, err := NewGraph("test", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-4")}
	err = g.Walk(context.Background(), w, "nonexistent")
	if !errors.Is(err, circuit.ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestGraph_ErrNodeNotFound_EdgeTarget(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []circuit.Edge{
		&stubEdge{id: "bad", from: "A", to: "Z"},
	}

	_, err := NewGraph("test", []circuit.Node{nodeA}, edges, nil)
	if !errors.Is(err, circuit.ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound during construction, got %v", err)
	}
}

func TestGraph_ErrNoEdge(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdgeWithEvalFn{
			id: "B-never", from: "B", to: "_done",
			evalFn: func(circuit.Artifact, *circuit.WalkerState) *circuit.Transition { return nil },
		},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-5")}
	err = g.Walk(context.Background(), w, "A")
	if !errors.Is(err, circuit.ErrNoEdge) {
		t.Errorf("expected ErrNoEdge, got %v", err)
	}
	if w.state.Status != "error" {
		t.Errorf("expected status error, got %s", w.state.Status)
	}
}

func TestGraph_TerminalNodeNoEdges(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}

	g, err := NewGraph("test", []circuit.Node{nodeA}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-6")}
	err = g.Walk(context.Background(), w, "A")
	if err != nil {
		t.Fatalf("expected nil error for terminal node, got %v", err)
	}
	if w.state.Status != "done" {
		t.Errorf("expected status done, got %s", w.state.Status)
	}
}

func TestGraph_ContextCancellation(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-7")}
	err = g.Walk(ctx, w, "A")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if w.state.Status != "error" {
		t.Errorf("expected status error, got %s", w.state.Status)
	}
}

func TestGraph_Zones(t *testing.T) {
	nodeA := &stubNode{name: "A"}
	nodeB := &stubNode{name: "B"}
	nodeC := &stubNode{name: "C"}

	zones := []Zone{
		{Name: "front", NodeNames: []string{"A", "B"}, Approach: "fire", Stickiness: 0},
		{Name: "back", NodeNames: []string{"C"}, Approach: "water", Stickiness: 3},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB, nodeC}, nil, zones)
	if err != nil {
		t.Fatal(err)
	}

	if len(g.Zones()) != 2 {
		t.Errorf("expected 2 zones, got %d", len(g.Zones()))
	}
	if g.Zones()[0].Name != "front" {
		t.Errorf("expected zone 'front', got %q", g.Zones()[0].Name)
	}
}

func TestGraph_ContextAdditions(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []circuit.Edge{
		&stubEdgeWithEvalFn{
			id: "A-B", from: "A", to: "B",
			evalFn: func(circuit.Artifact, *circuit.WalkerState) *circuit.Transition {
				return &circuit.Transition{
					NextNode:         "B",
					ContextAdditions: map[string]any{"key": "value"},
					Explanation:      "with context",
				}
			},
		},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []circuit.Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-8")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatal(err)
	}

	if v, ok := w.state.Context["key"]; !ok || v != "value" {
		t.Errorf("expected context key=value, got %v", w.state.Context)
	}
}

func TestEvidenceSNR(t *testing.T) {
	cases := []struct {
		in, out int
		want    float64
	}{
		{10, 5, 0.5},
		{10, 10, 1.0},
		{10, 0, 0.0},
		{0, 5, 0.0},
		{0, 0, 0.0},
		{4, 3, 0.75},
	}
	for _, tc := range cases {
		got := evidenceSNR(tc.in, tc.out)
		if got != tc.want {
			t.Errorf("evidenceSNR(%d, %d) = %f, want %f", tc.in, tc.out, got, tc.want)
		}
	}
}

func TestWalk_SlowNode_ContextDeadline(t *testing.T) {
	nodeA := &slowNode{name: "slow", duration: 1 * time.Second}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "slow", to: "_done"}}

	g, err := NewGraph("timeout-test", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-timeout")}
	start := time.Now()
	err = g.Walk(ctx, w, "slow")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("walk took %v, expected ~100ms abort", elapsed)
	}
	if w.state.Status != "error" {
		t.Errorf("expected status 'error', got %q", w.state.Status)
	}
}

func TestWalk_CancelDuringNodeProcess(t *testing.T) {
	nodeA := &slowNode{name: "blocking", duration: 10 * time.Second}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "blocking", to: "_done"}}

	g, err := NewGraph("cancel-test", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-cancel")}
	start := time.Now()
	err = g.Walk(ctx, w, "blocking")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context canceled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("walk took %v, expected ~50ms abort", elapsed)
	}
}

func TestWalk_PerNodeTimeout_Enforced(t *testing.T) {
	nodeA := &slowNode{name: "slow-a", duration: 2 * time.Second}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "slow-a", to: "_done"}}

	g, err := NewGraph("per-node-timeout", []circuit.Node{nodeA}, edges, nil,
		WithNodeTimeouts(map[string]time.Duration{
			"slow-a": 100 * time.Millisecond,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-pnt")}
	start := time.Now()
	err = g.Walk(context.Background(), w, "slow-a")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("walk took %v, expected ~100ms abort", elapsed)
	}
}

func TestWalk_PerNodeTimeout_FastNodeSucceeds(t *testing.T) {
	nodeA := &stubNode{name: "fast", artifact: &stubArtifact{typ: "ok"}}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "fast", to: "_done"}}

	g, err := NewGraph("fast-ok", []circuit.Node{nodeA}, edges, nil,
		WithNodeTimeouts(map[string]time.Duration{
			"fast": 5 * time.Second,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-fast")}
	if err := g.Walk(context.Background(), w, "fast"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if w.state.Status != "done" {
		t.Errorf("expected done, got %s", w.state.Status)
	}
}

func TestWalk_PerNodeTimeout_OnlyAffectsTaggedNode(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &slowNode{name: "B", duration: 200 * time.Millisecond}
	edges := []circuit.Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("selective-timeout", []circuit.Node{nodeA, nodeB}, edges, nil,
		WithNodeTimeouts(map[string]time.Duration{
			"A": 50 * time.Millisecond,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("case-selective")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("expected success (B has no timeout), got: %v", err)
	}
	if w.state.Status != "done" {
		t.Errorf("expected done, got %s", w.state.Status)
	}
}

func TestWalk_SNRAutoEmitted(t *testing.T) {
	art := &countableStubArtifact{typ: "filtered", confidence: 0.9, inputN: 100, outputN: 30}
	nodeA := &stubNode{name: "filter", artifact: art}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "filter", to: "_done"}}

	tc := &TraceCollector{}
	g, err := NewGraph("snr-test", []circuit.Node{nodeA}, edges, nil, WithObserver(tc))
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{
		identity: circuit.AgentIdentity{Name: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	if err := g.Walk(context.Background(), w, "filter"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	exits := tc.EventsOfType(circuit.EventNodeExit)
	if len(exits) != 1 {
		t.Fatalf("expected 1 node_exit, got %d", len(exits))
	}

	snrVal, ok := exits[0].Metadata["snr"].(float64)
	if !ok {
		t.Fatal("EventNodeExit missing snr metadata")
	}
	want := 0.3
	if snrVal != want {
		t.Errorf("snr = %f, want %f", snrVal, want)
	}
}

func TestWalk_SNRNotEmittedForNonCountable(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "plain", confidence: 0.5}}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "A", to: "_done"}}

	tc := &TraceCollector{}
	g, err := NewGraph("no-snr", []circuit.Node{nodeA}, edges, nil, WithObserver(tc))
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{
		identity: circuit.AgentIdentity{Name: "Solo"},
		state:    circuit.NewWalkerState("s1"),
	}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	exits := tc.EventsOfType(circuit.EventNodeExit)
	if len(exits) != 1 {
		t.Fatalf("expected 1 node_exit, got %d", len(exits))
	}

	if _, ok := exits[0].Metadata["snr"]; ok {
		t.Error("non-CountableArtifact should not emit snr metadata")
	}
}

func TestWalk_CircuitMission_InjectedIntoContext(t *testing.T) {
	t.Parallel()

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "A", to: "_done"}}

	g, err := NewGraph("mission-test", []circuit.Node{nodeA}, edges, nil,
		WithDescription("Scan, fix, and deploy Go microservices"))
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("w1")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	got, ok := w.state.Context["circuit_mission"]
	if !ok {
		t.Fatal("circuit_mission not found in walker context")
	}
	if got != "Scan, fix, and deploy Go microservices" {
		t.Errorf("circuit_mission = %q, want %q", got, "Scan, fix, and deploy Go microservices")
	}
}

func TestWalk_CircuitMission_EmptyDescription_NotInjected(t *testing.T) {
	t.Parallel()

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	edges := []circuit.Edge{&stubEdge{id: "A-done", from: "A", to: "_done"}}

	g, err := NewGraph("no-mission", []circuit.Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &visitTrackingWalker{state: circuit.NewWalkerState("w2")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if _, ok := w.state.Context["circuit_mission"]; ok {
		t.Error("circuit_mission should not be set when description is empty")
	}
}

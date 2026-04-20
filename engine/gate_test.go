package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/gate"
)

// testApprovalStore is a minimal in-memory ApprovalStore for engine-internal tests.
type testApprovalStore struct {
	mu    sync.Mutex
	items map[string]*gate.ApprovalItem
}

func newTestApprovalStore() *testApprovalStore {
	return &testApprovalStore{items: make(map[string]*gate.ApprovalItem)}
}

func (s *testApprovalStore) Park(_ context.Context, item gate.ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

func (s *testApprovalStore) Get(_ context.Context, id string) (*gate.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

func (s *testApprovalStore) List(_ context.Context, status gate.ApprovalStatus) ([]gate.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []gate.ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

func (s *testApprovalStore) Resolve(_ context.Context, id string, decision gate.Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return nil
}

func (s *testApprovalStore) AddComment(_ context.Context, id string, comment gate.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	item.Comments = append(item.Comments, comment)
	return nil
}

// testNotifier records notification calls.
type testNotifier struct {
	mu    sync.Mutex
	calls []gate.ApprovalItem
	err   error
}

func (n *testNotifier) Notify(_ context.Context, item gate.ApprovalItem) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, item)
	return n.err
}

func (n *testNotifier) callCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

func TestWalk_GatedNode_Interrupts(t *testing.T) {
	// RED: circuit with gate: approval on a node.
	// Walk should interrupt after the node produces its artifact.
	// The artifact should be parked in the ApprovalStore.

	def := &circuit.CircuitDef{
		Circuit: "gate-test",
		Start:   "review",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "review", Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "review-done", From: "review", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	reg := &GraphRegistries{}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Attach approval store to graph.
	dg := g.(*DefaultGraph)
	dg.approvalStore = store

	walker := circuit.NewProcessWalker("test")
	walkErr := g.Walk(context.Background(), walker, "review")

	// Walk returns ErrWalkInterrupted for gated nodes.
	if !errors.Is(walkErr, ErrWalkInterrupted) {
		t.Fatalf("Walk: expected ErrWalkInterrupted, got %v", walkErr)
	}

	// Verify artifact was parked.
	pending, err := store.List(context.Background(), gate.ApprovalPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if pending[0].NodeName != "review" {
		t.Errorf("NodeName = %q, want review", pending[0].NodeName)
	}
}

func TestWalk_UngatedNode_NoInterrupt(t *testing.T) {
	// Ungated nodes should walk normally — no interrupt, no parking.

	def := &circuit.CircuitDef{
		Circuit: "no-gate-test",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-done", From: "process", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	reg := &GraphRegistries{}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	dg := g.(*DefaultGraph)
	dg.approvalStore = store

	walker := circuit.NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "process"); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// No items should be parked.
	pending, _ := store.List(context.Background(), gate.ApprovalPending)
	if len(pending) != 0 {
		t.Errorf("pending = %d, want 0 for ungated node", len(pending))
	}
}

func TestWalk_GatedNode_NotifiesSent(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "notify-test",
		Start:   "deploy",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	notifier := &testNotifier{}
	reg := &GraphRegistries{}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	dg := g.(*DefaultGraph)
	dg.approvalStore = store
	dg.approvalNotifier = notifier

	walker := circuit.NewProcessWalker("test")
	g.Walk(context.Background(), walker, "deploy")

	if notifier.callCount() != 1 {
		t.Errorf("notifier calls = %d, want 1", notifier.callCount())
	}
}

func TestWalk_GatedNode_InDelegateSubWalk_Interrupts(t *testing.T) {
	// A gate: approval node inside a delegated sub-circuit must propagate
	// as ErrWalkInterrupted to the outer walk, not as a node failure.

	innerDef := &circuit.CircuitDef{
		Circuit: "publishing",
		Start:   "diff-review",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "diff-review", Instrument: InstrumentTransformer, Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "review-done", From: "diff-review", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "main",
		Start:   "code",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "code", Instrument: InstrumentTransformer, Action: "passthrough"},
			{Name: "publish", Instrument: InstrumentCircuit, Action: "publishing"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "code-publish", From: "code", To: "publish"},
			{ID: "publish-done", From: "publish", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits:      map[string]*circuit.CircuitDef{"publishing": innerDef},
		ApprovalStore: store,
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
	walkErr := g.Walk(context.Background(), walker, "code")

	if !errors.Is(walkErr, ErrWalkInterrupted) {
		t.Fatalf("Walk: expected ErrWalkInterrupted, got %v", walkErr)
	}

	// Verify artifact was parked in the sub-walk's approval store.
	pending, err := store.List(context.Background(), gate.ApprovalPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if pending[0].NodeName != "diff-review" {
		t.Errorf("NodeName = %q, want diff-review", pending[0].NodeName)
	}
}

func TestWalk_GatedNode_InDelegateSubWalk_LogsGatePark(t *testing.T) {
	// A gate interrupt in a delegate sub-walk must emit an INFO log so
	// operators can see why the walk stopped.

	innerDef := &circuit.CircuitDef{
		Circuit: "publishing",
		Start:   "diff-review",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "diff-review", Instrument: InstrumentTransformer, Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "review-done", From: "diff-review", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "main",
		Start:   "publish",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "publish", Instrument: InstrumentCircuit, Action: "publishing"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "publish-done", From: "publish", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits:      map[string]*circuit.CircuitDef{"publishing": innerDef},
		ApprovalStore: store,
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Capture logs.
	var buf logBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	walker := circuit.NewProcessWalker("test")
	g.Walk(context.Background(), walker, "publish")

	if !buf.contains("delegate sub-walk parked at gate") {
		t.Errorf("expected log message %q, got:\n%s", "delegate sub-walk parked at gate", buf.String())
	}
}

// logBuffer is a thread-safe bytes.Buffer for capturing slog output in tests.
type logBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (b *logBuffer) contains(s string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf) > 0 && strings.Contains(string(b.buf), s)
}

func TestWalk_GatedNode_NotifierError_DoesNotBlockPark(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "notify-error-test",
		Start:   "deploy",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	store := newTestApprovalStore()
	notifier := &testNotifier{}
	notifier.err = errors.New("slack down")
	reg := &GraphRegistries{}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	dg := g.(*DefaultGraph)
	dg.approvalStore = store
	dg.approvalNotifier = notifier

	walker := circuit.NewProcessWalker("test")
	g.Walk(context.Background(), walker, "deploy")

	// Item should still be parked even if notification fails.
	pending, _ := store.List(context.Background(), gate.ApprovalPending)
	if len(pending) != 1 {
		t.Errorf("pending = %d, want 1 even when notifier fails", len(pending))
	}
}

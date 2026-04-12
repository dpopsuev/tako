package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// testApprovalStore is a minimal in-memory ApprovalStore for engine-internal tests.
type testApprovalStore struct {
	mu    sync.Mutex
	items map[string]*ApprovalItem
}

func newTestApprovalStore() *testApprovalStore {
	return &testApprovalStore{items: make(map[string]*ApprovalItem)}
}

func (s *testApprovalStore) Park(_ context.Context, item ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

func (s *testApprovalStore) Get(_ context.Context, id string) (*ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

func (s *testApprovalStore) List(_ context.Context, status ApprovalStatus) ([]ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

func (s *testApprovalStore) Resolve(_ context.Context, id string, decision Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return nil
}

// testNotifier records notification calls.
type testNotifier struct {
	mu    sync.Mutex
	calls []ApprovalItem
	err   error
}

func (n *testNotifier) Notify(_ context.Context, item ApprovalItem) error {
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
			{Name: "review", Instrument: "transformer", Action: "passthrough", Gate: GateApproval},
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
	pending, err := store.List(context.Background(), ApprovalPending)
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
	pending, _ := store.List(context.Background(), ApprovalPending)
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
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: GateApproval},
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

func TestWalk_GatedNode_NotifierError_DoesNotBlockPark(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "notify-error-test",
		Start:   "deploy",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: GateApproval},
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
	pending, _ := store.List(context.Background(), ApprovalPending)
	if len(pending) != 1 {
		t.Errorf("pending = %d, want 1 even when notifier fails", len(pending))
	}
}

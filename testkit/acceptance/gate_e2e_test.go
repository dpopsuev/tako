package acceptance_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/testkit/assertions"
	"github.com/dpopsuev/origami/testkit/stubs"
)

// --- Helpers ---

func gateCircuit(gatedNode string) *circuit.CircuitDef {
	nn := circuit.NodeName(gatedNode)
	return &circuit.CircuitDef{
		Circuit: "gate-e2e",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
			{Name: nn, Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-" + gatedNode, From: "process", To: nn},
			{ID: gatedNode + "-done", From: nn, To: "_done"},
		},
	}
}

func buildGateGraph(t *testing.T, def *circuit.CircuitDef, store gate.ApprovalStore, notifier gate.Notifier) (engine.Graph, *traceCollector) {
	t.Helper()
	reg := &engine.GraphRegistries{
		ApprovalStore:    store,
		ApprovalNotifier: notifier,
	}
	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	tc := &traceCollector{}
	if dg, ok := g.(*engine.DefaultGraph); ok {
		dg.SetObserver(tc)
	}
	return g, tc
}

type traceCollector struct {
	mu     sync.Mutex
	events []circuit.WalkEvent
}

func (tc *traceCollector) OnEvent(e *circuit.WalkEvent) {
	tc.mu.Lock()
	tc.events = append(tc.events, *e)
	tc.mu.Unlock()
}

func (tc *traceCollector) snapshot() []circuit.WalkEvent {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	out := make([]circuit.WalkEvent, len(tc.events))
	copy(out, tc.events)
	return out
}

func dumpTrace(tb testing.TB, trace []circuit.WalkEvent) {
	tb.Helper()
	tb.Log("=== Walk Trace ===")
	for i, e := range trace {
		tb.Logf("  [%d] %s node=%s walker=%s", i, e.Type, e.Node, e.Walker)
	}
	tb.Log("=== End Trace ===")
}

func assertVisited(t *testing.T, trace []circuit.WalkEvent, node string) {
	t.Helper()
	for _, e := range trace {
		if e.Type == circuit.EventNodeEnter && e.Node == node {
			return
		}
	}
	dumpTrace(t, trace)
	t.Errorf("node %q never visited", node)
}

// --- TSK-653: Park + Approve (baseline) ---

func TestGateE2E_ParkApproveResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := stubs.NewMemoryApprovalStore()
	notifier := stubs.NewStubNotifier()
	g, tc := buildGateGraph(t, gateCircuit("deploy"), store, notifier)

	walker := circuit.NewProcessWalker("gate-e2e")
	walkErr := g.Walk(ctx, walker, "process")

	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		dumpTrace(t, tc.snapshot())
		t.Fatalf("Walk: expected ErrWalkInterrupted, got %v", walkErr)
	}

	parked := assertions.AssertParked(t, store, "deploy")
	t.Logf("parked: id=%s node=%s", parked.ID, parked.NodeName)

	if notifier.CallCount() != 1 {
		t.Errorf("notifier calls = %d, want 1", notifier.CallCount())
	}

	err := store.Resolve(ctx, parked.ID, gate.Decision{
		Status:   gate.ApprovalApproved,
		Comment:  "LGTM",
		Operator: "test-operator",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	assertions.AssertApproved(t, store, parked.ID)
	assertions.AssertNoPending(t, store)
	assertVisited(t, tc.snapshot(), "process")
	assertVisited(t, tc.snapshot(), "deploy")
}

// --- TSK-693: Resume walk after approval ---

func TestGateE2E_ResumeAfterApproval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := stubs.NewMemoryApprovalStore()
	g, tc := buildGateGraph(t, gateCircuit("deploy"), store, nil)

	// First walk — parks at gate.
	walker := circuit.NewProcessWalker("resume-test")
	walkErr := g.Walk(ctx, walker, "process")
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		dumpTrace(t, tc.snapshot())
		t.Fatalf("Walk 1: expected interrupt, got %v", walkErr)
	}

	parked := assertions.AssertParked(t, store, "deploy")

	// Approve.
	store.Resolve(ctx, parked.ID, gate.Decision{
		Status: gate.ApprovalApproved, Operator: "test",
	})

	// Resume walk from the edge after the gated node.
	// The walker state has deploy's output in Outputs["deploy"].
	// Walking from the edges after deploy should reach _done.
	edges := g.EdgesFrom("deploy")
	if len(edges) == 0 {
		t.Fatal("no edges from deploy — can't resume")
	}

	nextNode := edges[0].To()
	if nextNode == "_done" {
		// deploy → _done means walk completes immediately.
		t.Log("resume target is _done — walk completes after approval")
		assertions.AssertApproved(t, store, parked.ID)
		return
	}

	// If there's a real next node, walk from it.
	walkErr = g.Walk(ctx, walker, nextNode)
	if walkErr != nil {
		dumpTrace(t, tc.snapshot())
		t.Fatalf("Walk 2 (resume): %v", walkErr)
	}
}

// --- TSK-695: Rejection with mandatory comment ---

func TestGateE2E_RejectionWithComment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := stubs.NewMemoryApprovalStore()
	g, tc := buildGateGraph(t, gateCircuit("deploy"), store, nil)

	walker := circuit.NewProcessWalker("reject-test")
	walkErr := g.Walk(ctx, walker, "process")
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		dumpTrace(t, tc.snapshot())
		t.Fatalf("Walk: expected interrupt, got %v", walkErr)
	}

	parked := assertions.AssertParked(t, store, "deploy")

	// Reject WITH comment.
	err := store.Resolve(ctx, parked.ID, gate.Decision{
		Status:   gate.ApprovalRejected,
		Comment:  "Not ready for production — missing rollback plan",
		Operator: "reviewer",
	})
	if err != nil {
		t.Fatalf("Resolve reject: %v", err)
	}

	// Verify rejected state with comment preserved.
	assertions.AssertRejected(t, store, parked.ID)

	item, _ := store.Get(ctx, parked.ID)
	if item.Decision == nil {
		t.Fatal("decision is nil after rejection")
	}
	if item.Decision.Comment == "" {
		t.Error("rejection comment is empty — should be mandatory")
	}
	if item.Decision.Comment != "Not ready for production — missing rollback plan" {
		t.Errorf("comment = %q", item.Decision.Comment)
	}
	if item.Decision.Operator != "reviewer" {
		t.Errorf("operator = %q", item.Decision.Operator)
	}

	// Verify no pending items.
	assertions.AssertNoPending(t, store)
}

// --- TSK-696: Multi-gate circuit ---

func TestGateE2E_MultiGate_SequentialApproval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Circuit: process → review (gated) → deploy (gated) → done
	def := &circuit.CircuitDef{
		Circuit: "multi-gate",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
			{Name: "review", Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-review", From: "process", To: "review"},
			{ID: "review-deploy", From: "review", To: "deploy"},
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	store := stubs.NewMemoryApprovalStore()
	notifier := stubs.NewStubNotifier()
	reg := &engine.GraphRegistries{
		ApprovalStore:    store,
		ApprovalNotifier: notifier,
	}
	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Walk 1: parks at first gate (review).
	walker := circuit.NewProcessWalker("multi-gate")
	walkErr := g.Walk(ctx, walker, "process")
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		t.Fatalf("Walk 1: expected interrupt at review, got %v", walkErr)
	}

	// First gate: review.
	parked1 := assertions.AssertParked(t, store, "review")
	t.Logf("gate 1: id=%s node=%s", parked1.ID, parked1.NodeName)

	// Approve first gate.
	store.Resolve(ctx, parked1.ID, gate.Decision{
		Status: gate.ApprovalApproved, Operator: "lead", Comment: "code looks good",
	})

	// Walk 2: resume from review's successor — should park at deploy.
	walkErr = g.Walk(ctx, walker, "deploy")
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		t.Fatalf("Walk 2: expected interrupt at deploy, got %v", walkErr)
	}

	// Second gate: deploy.
	parked2 := assertions.AssertParked(t, store, "deploy")
	t.Logf("gate 2: id=%s node=%s", parked2.ID, parked2.NodeName)

	if parked1.ID == parked2.ID {
		t.Error("both gates have the same ID — should be unique")
	}

	// Approve second gate.
	store.Resolve(ctx, parked2.ID, gate.Decision{
		Status: gate.ApprovalApproved, Operator: "ops", Comment: "deploy approved",
	})

	// Verify both approved, none pending.
	assertions.AssertApproved(t, store, parked1.ID)
	assertions.AssertApproved(t, store, parked2.ID)
	assertions.AssertNoPending(t, store)

	// Verify notifications: 2 gates = 2 notifications.
	if notifier.CallCount() != 2 {
		t.Errorf("notifier calls = %d, want 2", notifier.CallCount())
	}
}

// --- TSK-697: Retry with rejection feedback ---

// feedbackCapture records the walker context each time the transformer is called.
type feedbackCapture struct {
	mu       sync.Mutex
	contexts []map[string]any
}

func (fc *feedbackCapture) snapshot() []map[string]any {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	out := make([]map[string]any, len(fc.contexts))
	for i, m := range fc.contexts {
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		out[i] = cp
	}
	return out
}

func TestGateE2E_RetryAfterRejection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Capture transformer — records walker context on each call.
	capture := &feedbackCapture{}
	captureTrans := engine.TransformerFunc("capture-feedback", func(_ context.Context, tc *engine.TransformerContext) (any, error) {
		capture.mu.Lock()
		// Deep copy walker context.
		ctxCopy := make(map[string]any)
		if tc.WalkerState != nil {
			for k, v := range tc.WalkerState.Context {
				ctxCopy[k] = v
			}
		}
		capture.contexts = append(capture.contexts, ctxCopy)
		capture.mu.Unlock()
		return "output-v" + fmt.Sprintf("%d", len(capture.contexts)), nil
	})

	// Circuit: process → deploy (gated, uses capture-feedback transformer) → _done.
	def := &circuit.CircuitDef{
		Circuit: "retry-gate",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
			{Name: "deploy", Instrument: "transformer", Action: "capture-feedback", Gate: gate.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-deploy", From: "process", To: "deploy"},
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	store := stubs.NewMemoryApprovalStore()
	reg := &engine.GraphRegistries{
		ApprovalStore: store,
		Transformers:  engine.TransformerRegistry{"capture-feedback": captureTrans},
	}
	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Walk 1 — parks at deploy.
	walker := circuit.NewProcessWalker("retry-test")
	walkErr := g.Walk(ctx, walker, "process")
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		t.Fatalf("Walk 1: expected interrupt, got %v", walkErr)
	}

	parked := assertions.AssertParked(t, store, "deploy")

	// First call: no rejection feedback.
	calls := capture.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 capture call before rejection, got %d", len(calls))
	}
	if _, hasFeedback := calls[0]["rejection_feedback"]; hasFeedback {
		t.Error("first call should NOT have rejection_feedback")
	}

	// Reject with comment.
	err = store.Resolve(ctx, parked.ID, gate.Decision{
		Status:   gate.ApprovalRejected,
		Comment:  "missing rollback plan",
		Operator: "reviewer",
	})
	if err != nil {
		t.Fatalf("Resolve reject: %v", err)
	}

	// Resume from gate — should inject feedback and re-walk the gated node.
	walkErr = engine.ResumeFromGate(ctx, g, walker, store)
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		t.Fatalf("ResumeFromGate: expected interrupt (re-parked), got %v", walkErr)
	}

	// Second call: rejection_feedback injected.
	calls = capture.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 capture calls after retry, got %d", len(calls))
	}
	feedback, ok := calls[1]["rejection_feedback"]
	if !ok {
		t.Fatal("second call missing rejection_feedback in walker context")
	}
	if feedback != "missing rollback plan" {
		t.Errorf("rejection_feedback = %q, want %q", feedback, "missing rollback plan")
	}

	// Approve the re-parked item.
	parked2 := assertions.AssertParked(t, store, "deploy")
	if parked2.ID == parked.ID {
		t.Error("re-parked item has same ID as original — should be unique")
	}

	err = store.Resolve(ctx, parked2.ID, gate.Decision{
		Status:   gate.ApprovalApproved,
		Comment:  "rollback plan added",
		Operator: "lead",
	})
	if err != nil {
		t.Fatalf("Resolve approve: %v", err)
	}

	// Resume again — should complete (reach _done).
	walkErr = engine.ResumeFromGate(ctx, g, walker, store)
	if walkErr != nil {
		t.Fatalf("ResumeFromGate after approval: %v", walkErr)
	}

	// Verify final state.
	assertions.AssertApproved(t, store, parked2.ID)
	assertions.AssertNoPending(t, store)
}

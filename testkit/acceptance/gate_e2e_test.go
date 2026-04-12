package acceptance_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/assertions"
	"github.com/dpopsuev/origami/testkit/stubs"
)

// errWalkInterrupted is the expected error when a gated node parks output.
// The engine returns this as an unexported sentinel — we match by message.
const walkInterruptedMsg = "walk interrupted"

// TestGateE2E_ParkApproveResume verifies the full approval gate lifecycle:
// walk → gate interrupts → item parked → approve → verify state.
func TestGateE2E_ParkApproveResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Circuit: process → deploy (gated) → done
	def := &circuit.CircuitDef{
		Circuit: "gate-e2e",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: engine.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-deploy", From: "process", To: "deploy"},
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	store := stubs.NewMemoryApprovalStore()
	notifier := stubs.NewStubNotifier()

	// Capture walk events for trace dump on failure.
	var mu sync.Mutex
	var trace []circuit.WalkEvent
	observer := circuit.WalkObserverFunc(func(e *circuit.WalkEvent) {
		mu.Lock()
		trace = append(trace, *e)
		mu.Unlock()
	})

	reg := &engine.GraphRegistries{
		ApprovalStore:    store,
		ApprovalNotifier: notifier,
	}
	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Set observer on the built graph.
	if dg, ok := g.(*engine.DefaultGraph); ok {
		dg.SetObserver(observer)
	}

	// Walk — should interrupt at gated node.
	walker := circuit.NewProcessWalker("gate-e2e")
	walkErr := g.Walk(ctx, walker, "process")

	// Walk should return an interrupt error (gated node parks output).
	if walkErr == nil {
		dumpTrace(t, trace)
		t.Fatal("Walk: expected interrupt error for gated node, got nil")
	}
	if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
		// Try message match for unexported sentinels.
		if walkErr.Error() != walkInterruptedMsg {
			dumpTrace(t, trace)
			t.Fatalf("Walk: expected walk interrupted, got: %v", walkErr)
		}
	}

	// Verify parked state using assertion helpers.
	parked := assertions.AssertParked(t, store, "deploy")
	t.Logf("parked: id=%s node=%s", parked.ID, parked.NodeName)

	// Verify notification was sent.
	if notifier.CallCount() != 1 {
		t.Errorf("notifier calls = %d, want 1", notifier.CallCount())
	}

	// Approve the item.
	err = store.Resolve(ctx, parked.ID, engine.Decision{
		Status:   engine.ApprovalApproved,
		Comment:  "LGTM",
		Operator: "test-operator",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Verify approved state.
	assertions.AssertApproved(t, store, parked.ID)
	assertions.AssertNoPending(t, store)

	// Verify both nodes were visited in the trace.
	mu.Lock()
	traceSnapshot := make([]circuit.WalkEvent, len(trace))
	copy(traceSnapshot, trace)
	mu.Unlock()

	visitedProcess := false
	visitedDeploy := false
	for _, e := range traceSnapshot {
		if e.Type == circuit.EventNodeEnter && e.Node == "process" {
			visitedProcess = true
		}
		if e.Type == circuit.EventNodeEnter && e.Node == "deploy" {
			visitedDeploy = true
		}
	}
	if !visitedProcess {
		dumpTrace(t, trace)
		t.Error("node 'process' never visited")
	}
	if !visitedDeploy {
		dumpTrace(t, trace)
		t.Error("node 'deploy' never visited")
	}
}

// dumpTrace prints the full walk trace on failure for debugging.
func dumpTrace(tb testing.TB, trace []circuit.WalkEvent) {
	tb.Helper()
	tb.Log("=== Walk Trace ===")
	for i, e := range trace {
		tb.Logf("  [%d] %s node=%s walker=%s", i, e.Type, e.Node, e.Walker)
	}
	tb.Log("=== End Trace ===")
}

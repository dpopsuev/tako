package operator_test

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/origami/testkit/stubs"
	"github.com/dpopsuev/troupe/signal"
	"github.com/dpopsuev/troupe/world"
)

// TestIntegration_FullGateLifecycle exercises the complete operator flow:
//  1. Register operator entity in World
//  2. Wire edges to an agent
//  3. Park a gate item (simulating circuit walk)
//  4. Signal bridge emits gate.parked
//  5. CLIGatekeeper detects the parked item
//  6. Human resolves via store
//  7. Gatekeeper returns decision
func TestIntegration_FullGateLifecycle(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	bus := signal.NewMemBus()
	store := stubs.NewMemoryApprovalStore()

	// 1. Register operator.
	opID := operator.RegisterOperator(w, "Alice")
	agentID := w.Spawn()

	// 2. Wire edges.
	err := operator.WireOperatorEdges(w, opID, []world.EntityID{agentID})
	if err != nil {
		t.Fatalf("WireOperatorEdges: %v", err)
	}

	// Verify bidirectional edges.
	opNeighbors := w.Neighbors(opID, world.CommunicatesWith, world.Outbound)
	if len(opNeighbors) != 1 || opNeighbors[0] != agentID {
		t.Fatalf("operator neighbors = %v, want [%d]", opNeighbors, agentID)
	}

	// 3. Set up signal notifier (what the engine would use).
	notifier := &operator.SignalNotifier{Bus: bus}

	// 4. Simulate circuit parking a gate item.
	item := gate.ApprovalItem{
		ID:         "walk-1:diff-review:1",
		CircuitRun: "walk-1",
		NodeName:   "diff-review",
		ParkedAt:   time.Now(),
		Status:     gate.ApprovalPending,
	}
	if err := store.Park(context.Background(), item); err != nil {
		t.Fatalf("Park: %v", err)
	}
	if err := notifier.Notify(context.Background(), item); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	// 5. Verify signal was emitted.
	signals := bus.Since(0)
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}
	if signals[0].Event != operator.EventGateParked {
		t.Errorf("signal event = %q, want gate.parked", signals[0].Event)
	}
	if signals[0].Meta[operator.MetaKeyNodeName] != "diff-review" {
		t.Errorf("signal node_name = %q, want diff-review", signals[0].Meta[operator.MetaKeyNodeName])
	}

	// 6. Set up CLIGatekeeper and resolve from background (simulating human).
	gk := &operator.CLIGatekeeper{
		Store:    store,
		Bus:      bus,
		Operator: "Alice",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Human approves after short delay — resolve ALL pending items.
	go func() {
		time.Sleep(200 * time.Millisecond)
		// Resolve the engine-parked item.
		_ = store.Resolve(ctx, "walk-1:diff-review:1", gate.Decision{
			Status:   gate.ApprovalApproved,
			Comment:  "ship it",
			Operator: "Alice",
		})
		// Resolve whatever the CLIGatekeeper parked.
		items, _ := store.List(ctx, gate.ApprovalPending)
		for _, it := range items {
			_ = store.Resolve(ctx, it.ID, gate.Decision{
				Status:   gate.ApprovalApproved,
				Comment:  "ship it",
				Operator: "Alice",
			})
		}
	}()

	// 7. Gatekeeper blocks until resolved.
	// Note: CLIGatekeeper.Pass parks its OWN item (separate from the engine-parked one).
	// In production, the engine parks and the CLI resolves. Here we test CLIGatekeeper's
	// own park+poll lifecycle separately, and verify the engine-parked item above.
	allowed, comment, err := gk.Pass(ctx, `{"diff": "reviewed changes"}`)
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed=true")
	}

	// Verify the engine-parked item was resolved by our goroutine.
	resolved, err := store.Get(ctx, "walk-1:diff-review:1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resolved.Status != gate.ApprovalApproved {
		t.Errorf("engine item status = %q, want approved", resolved.Status)
	}
	if resolved.Decision.Comment != "ship it" {
		t.Errorf("decision comment = %q, want 'ship it'", resolved.Decision.Comment)
	}

	// Verify all signals (gate.parked from notifier + gate.parked from CLIGatekeeper).
	allSignals := bus.Since(0)
	gateParkedCount := 0
	for _, s := range allSignals {
		if s.Event == operator.EventGateParked {
			gateParkedCount++
		}
	}
	if gateParkedCount < 2 {
		t.Errorf("gate.parked signals = %d, want >= 2 (notifier + gatekeeper)", gateParkedCount)
	}

	_ = comment
}

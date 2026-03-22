package view

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// TestIntegration_WalkSequence simulates a full walk through a circuit
// and verifies that CircuitStore produces the correct StateDiff sequence
// and final snapshot.
func TestIntegration_WalkSequence(t *testing.T) {
	def := testCircuitDef()
	store := NewCircuitStore(def)
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"},
		{Type: circuit.EventNodeExit, Node: "recall", Walker: "w1"},
		{Type: circuit.EventTransition, Node: "triage", Edge: "e1", Walker: "w1"},
		{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1"},
		{Type: circuit.EventNodeExit, Node: "triage", Walker: "w1"},
		{Type: circuit.EventTransition, Node: "investigate", Edge: "e2", Walker: "w1"},
		{Type: circuit.EventNodeEnter, Node: "investigate", Walker: "w1"},
		{Type: circuit.EventNodeExit, Node: "investigate", Walker: "w1"},
		{Type: circuit.EventTransition, Node: "report", Edge: "e3", Walker: "w1"},
		{Type: circuit.EventNodeEnter, Node: "report", Walker: "w1"},
		{Type: circuit.EventNodeExit, Node: "report", Walker: "w1"},
		{Type: circuit.EventWalkComplete},
	}

	for _, e := range events {
		store.OnEvent(e)
	}

	diffs := collectDiffs(ch, 100*time.Millisecond)

	snap := store.Snapshot()

	// All nodes should be completed
	for _, name := range []string{"recall", "triage", "investigate", "report"} {
		if snap.Nodes[name].State != NodeCompleted {
			t.Errorf("node %q state = %q, want %q", name, snap.Nodes[name].State, NodeCompleted)
		}
	}

	if !snap.Completed {
		t.Error("circuit should be completed")
	}

	// Walker should have been added once and moved through all nodes
	walkerAddedCount := 0
	walkerMovedCount := 0
	nodeStateCount := 0
	completedCount := 0
	for _, d := range diffs {
		switch d.Type {
		case DiffWalkerAdded:
			walkerAddedCount++
		case DiffWalkerMoved:
			walkerMovedCount++
		case DiffNodeState:
			nodeStateCount++
		case DiffCompleted:
			completedCount++
		}
	}

	if walkerAddedCount != 1 {
		t.Errorf("walker_added count = %d, want 1", walkerAddedCount)
	}
	if walkerMovedCount != 3 {
		t.Errorf("walker_moved count = %d, want 3 (triage, investigate, report)", walkerMovedCount)
	}
	// 4 nodes x 2 events (enter=active, exit=completed) = 8 node_state diffs
	if nodeStateCount != 8 {
		t.Errorf("node_state count = %d, want 8", nodeStateCount)
	}
	if completedCount != 1 {
		t.Errorf("completed count = %d, want 1", completedCount)
	}
}

// TestIntegration_WalkWithError simulates a walk that fails at a node.
func TestIntegration_WalkWithError(t *testing.T) {
	def := testCircuitDef()
	store := NewCircuitStore(def)
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall", Walker: "w1"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1"})
	store.OnEvent(circuit.WalkEvent{
		Type:   circuit.EventWalkError,
		Node:   "triage",
		Walker: "w1",
		Error:  errTimeout{},
	})

	diffs := collectDiffs(ch, 100*time.Millisecond)
	snap := store.Snapshot()

	if snap.Nodes["triage"].State != NodeError {
		t.Errorf("triage state = %q, want %q", snap.Nodes["triage"].State, NodeError)
	}
	if snap.Error != "timeout" {
		t.Errorf("error = %q, want %q", snap.Error, "timeout")
	}
	if snap.Nodes["recall"].State != NodeCompleted {
		t.Errorf("recall state = %q, want %q", snap.Nodes["recall"].State, NodeCompleted)
	}
	if snap.Nodes["investigate"].State != NodeIdle {
		t.Errorf("investigate state = %q, want %q (never visited)", snap.Nodes["investigate"].State, NodeIdle)
	}

	var hasError bool
	for _, d := range diffs {
		if d.Type == DiffError && d.Node == "triage" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error diff for triage")
	}
}

type errTimeout struct{}

func (errTimeout) Error() string { return "timeout" }

// TestIntegration_TwoSubscribers verifies that two subscribers receive
// the same diffs independently.
func TestIntegration_TwoSubscribers(t *testing.T) {
	def := testCircuitDef()
	store := NewCircuitStore(def)

	id1, ch1 := store.Subscribe()
	id2, ch2 := store.Subscribe()
	defer store.Unsubscribe(id1)
	defer store.Unsubscribe(id2)

	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	diffs1 := collectDiffs(ch1, 50*time.Millisecond)
	diffs2 := collectDiffs(ch2, 50*time.Millisecond)

	if len(diffs1) != len(diffs2) {
		t.Errorf("subscriber 1 got %d diffs, subscriber 2 got %d", len(diffs1), len(diffs2))
	}

	for i := range diffs1 {
		if i < len(diffs2) && diffs1[i].Type != diffs2[i].Type {
			t.Errorf("diff[%d] type mismatch: %q vs %q", i, diffs1[i].Type, diffs2[i].Type)
		}
	}
}

// TestIntegration_StoreWithLayout verifies that CircuitStore and
// GridLayout work together on the same CircuitDef.
func TestIntegration_StoreWithLayout(t *testing.T) {
	def := testCircuitDef()
	store := NewCircuitStore(def)

	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	snap := store.Snapshot()

	// Every node in the snapshot should have a grid position
	for name := range snap.Nodes {
		if _, ok := layout.Grid[name]; !ok {
			t.Errorf("node %q in snapshot but not in layout", name)
		}
	}

	// Every node in the layout should be in the snapshot
	for name := range layout.Grid {
		if _, ok := snap.Nodes[name]; !ok {
			t.Errorf("node %q in layout but not in snapshot", name)
		}
	}
}

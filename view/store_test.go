package view

import (
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func testCircuitDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "test-circuit",
		Start:   "recall",
		Done:    "report",
		Zones: map[string]circuit.ZoneDef{
			"analysis": {Nodes: []circuit.NodeName{"recall", "triage"}, Approach: "rapid"},
			"output":   {Nodes: []circuit.NodeName{"investigate", "report"}, Approach: "analytical"},
		},
		Nodes: []circuit.NodeDef{
			{Name: "recall", Approach: "rapid"},
			{Name: "triage", Approach: "rapid"},
			{Name: "investigate", Approach: "analytical"},
			{Name: "report", Approach: "analytical"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", Name: "start", From: "recall", To: "triage"},
			{ID: "e2", Name: "analyze", From: "triage", To: "investigate"},
			{ID: "e3", Name: "conclude", From: "investigate", To: "report"},
		},
	}
}

func collectDiffs(ch <-chan StateDiff, timeout time.Duration) []StateDiff {
	var diffs []StateDiff
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case d, ok := <-ch:
			if !ok {
				return diffs
			}
			diffs = append(diffs, d)
		case <-timer.C:
			return diffs
		}
	}
}

func TestNewCircuitStore(t *testing.T) {
	def := testCircuitDef()
	store := NewCircuitStore(def)
	snap := store.Snapshot()

	if snap.CircuitName != "test-circuit" {
		t.Errorf("CircuitName = %q, want %q", snap.CircuitName, "test-circuit")
	}
	if len(snap.Nodes) != 4 {
		t.Fatalf("len(Nodes) = %d, want 4", len(snap.Nodes))
	}
	for _, name := range []string{"recall", "triage", "investigate", "report"} {
		ns, ok := snap.Nodes[name]
		if !ok {
			t.Errorf("node %q not found", name)
			continue
		}
		if ns.State != NodeIdle {
			t.Errorf("node %q state = %q, want %q", name, ns.State, NodeIdle)
		}
	}
	if snap.Nodes["recall"].Zone != "analysis" {
		t.Errorf("recall zone = %q, want %q", snap.Nodes["recall"].Zone, "analysis")
	}
	if snap.Nodes["report"].Zone != "output" {
		t.Errorf("report zone = %q, want %q", snap.Nodes["report"].Zone, "output")
	}
	if snap.Nodes["recall"].Element != "fire" {
		t.Errorf("recall element = %q, want %q", snap.Nodes["recall"].Element, "fire")
	}
	if len(snap.Walkers) != 0 {
		t.Errorf("len(Walkers) = %d, want 0", len(snap.Walkers))
	}
	if snap.Completed || snap.Paused {
		t.Error("new store should not be completed or paused")
	}
}

func TestCircuitStore_NodeEnter(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if snap.Nodes["recall"].State != NodeActive {
		t.Errorf("recall state = %q, want %q", snap.Nodes["recall"].State, NodeActive)
	}
	if wp, ok := snap.Walkers["w1"]; !ok {
		t.Error("walker w1 not found")
	} else if wp.Node != "recall" {
		t.Errorf("walker w1 node = %q, want %q", wp.Node, "recall")
	}

	if len(diffs) < 2 {
		t.Fatalf("got %d diffs, want >= 2 (node_state + walker_added)", len(diffs))
	}
	if diffs[0].Type != DiffNodeState || diffs[0].Node != "recall" || diffs[0].State != NodeActive {
		t.Errorf("diff[0] = %+v, want node_state/recall/active", diffs[0])
	}
	if diffs[1].Type != DiffWalkerAdded || diffs[1].Walker != "w1" {
		t.Errorf("diff[1] = %+v, want walker_added/w1", diffs[1])
	}
}

func TestCircuitStore_NodeExit(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall", Walker: "w1"})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if snap.Nodes["recall"].State != NodeCompleted {
		t.Errorf("recall state = %q, want %q", snap.Nodes["recall"].State, NodeCompleted)
	}
	if len(diffs) < 1 || diffs[0].Type != DiffNodeState || diffs[0].State != NodeCompleted {
		t.Errorf("expected node_state/completed diff, got %+v", diffs)
	}
}

func TestCircuitStore_WalkerMoved(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1"})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if snap.Walkers["w1"].Node != "triage" {
		t.Errorf("walker w1 at %q, want %q", snap.Walkers["w1"].Node, "triage")
	}

	var walkerDiff *StateDiff
	for i := range diffs {
		if diffs[i].Type == DiffWalkerMoved {
			walkerDiff = &diffs[i]
			break
		}
	}
	if walkerDiff == nil {
		t.Fatal("no walker_moved diff found")
	}
	if walkerDiff.Walker != "w1" || walkerDiff.Node != "triage" {
		t.Errorf("walker_moved diff = %+v, want w1/triage", walkerDiff)
	}
}

func TestCircuitStore_WalkComplete(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if !snap.Completed {
		t.Error("expected completed = true")
	}
	if len(diffs) != 1 || diffs[0].Type != DiffCompleted {
		t.Errorf("expected completed diff, got %+v", diffs)
	}
}

func TestCircuitStore_WalkError(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Node:  "triage",
		Error: errors.New("timeout"),
	})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if snap.Error != "timeout" {
		t.Errorf("error = %q, want %q", snap.Error, "timeout")
	}
	if snap.Nodes["triage"].State != NodeError {
		t.Errorf("triage state = %q, want %q", snap.Nodes["triage"].State, NodeError)
	}
	var errorDiff *StateDiff
	for i := range diffs {
		if diffs[i].Type == DiffError {
			errorDiff = &diffs[i]
		}
	}
	if errorDiff == nil || errorDiff.Error != "timeout" {
		t.Errorf("expected error diff with message, got %+v", errorDiff)
	}
}

func TestCircuitStore_WalkInterruptedResumed(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkInterrupted, Node: "triage"})

	if snap := store.Snapshot(); !snap.Paused {
		t.Error("expected paused after interrupted")
	}

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkResumed})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	if snap := store.Snapshot(); snap.Paused {
		t.Error("expected not paused after resumed")
	}

	var hasPaused, hasResumed bool
	for _, d := range diffs {
		if d.Type == DiffPaused {
			hasPaused = true
		}
		if d.Type == DiffResumed {
			hasResumed = true
		}
	}
	if !hasPaused || !hasResumed {
		t.Errorf("expected paused and resumed diffs, got %+v", diffs)
	}
}

func TestCircuitStore_FanOutStartEnd(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventFanOutStart, Walker: "w2", Node: "triage"})
	snap := store.Snapshot()
	if _, ok := snap.Walkers["w2"]; !ok {
		t.Error("walker w2 should be added on fan_out_start")
	}

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventFanOutEnd, Walker: "w2"})
	snap = store.Snapshot()
	if _, ok := snap.Walkers["w2"]; ok {
		t.Error("walker w2 should be removed on fan_out_end")
	}

	diffs := collectDiffs(ch, 50*time.Millisecond)
	var hasAdded, hasRemoved bool
	for _, d := range diffs {
		if d.Type == DiffWalkerAdded && d.Walker == "w2" {
			hasAdded = true
		}
		if d.Type == DiffWalkerRemoved && d.Walker == "w2" {
			hasRemoved = true
		}
	}
	if !hasAdded || !hasRemoved {
		t.Errorf("expected walker_added and walker_removed diffs, got %+v", diffs)
	}
}

func TestCircuitStore_Transition(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventTransition, Node: "triage", Edge: "e1"})
	diffs := collectDiffs(ch, 50*time.Millisecond)

	// Transition events are informational — no state change expected
	if len(diffs) != 0 {
		t.Errorf("transition should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_EdgeEvaluate(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventEdgeEvaluate, Edge: "e1"})
	diffs := collectDiffs(ch, 50*time.Millisecond)

	if len(diffs) != 0 {
		t.Errorf("edge_evaluate should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_WalkerSwitch(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkerSwitch, Walker: "w1", Node: "triage"})
	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap := store.Snapshot()
	if snap.Walkers["w1"].Node != "triage" {
		t.Errorf("walker w1 at %q, want %q", snap.Walkers["w1"].Node, "triage")
	}
	var hasMoved bool
	for _, d := range diffs {
		if d.Type == DiffWalkerMoved && d.Walker == "w1" {
			hasMoved = true
		}
	}
	if !hasMoved {
		t.Error("expected walker_moved diff on walker_switch")
	}
}

func TestCircuitStore_CheckpointSaved(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventCheckpointSaved})
	diffs := collectDiffs(ch, 50*time.Millisecond)
	if len(diffs) != 0 {
		t.Errorf("checkpoint_saved should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_ProviderFallback(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventProviderFallback})
	diffs := collectDiffs(ch, 50*time.Millisecond)
	if len(diffs) != 0 {
		t.Errorf("provider_fallback should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_CircuitBreakerEvents(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventCircuitOpen})
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventCircuitClose})
	diffs := collectDiffs(ch, 50*time.Millisecond)
	if len(diffs) != 0 {
		t.Errorf("circuit breaker events should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_RateLimitAndThermal(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventRateLimit})
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventThermalWarning})
	diffs := collectDiffs(ch, 50*time.Millisecond)
	if len(diffs) != 0 {
		t.Errorf("rate_limit/thermal should emit 0 diffs, got %d", len(diffs))
	}
}

func TestCircuitStore_SetBreakpoints(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.SetBreakpoints([]string{"recall", "triage"})
	snap := store.Snapshot()
	if !snap.Breakpoints["recall"] || !snap.Breakpoints["triage"] {
		t.Error("breakpoints not set")
	}

	store.SetBreakpoints([]string{"triage"})
	snap = store.Snapshot()
	if snap.Breakpoints["recall"] {
		t.Error("recall breakpoint should be cleared")
	}
	if !snap.Breakpoints["triage"] {
		t.Error("triage breakpoint should remain")
	}

	diffs := collectDiffs(ch, 50*time.Millisecond)
	var hasSet, hasCleared bool
	for _, d := range diffs {
		if d.Type == DiffBreakpointSet {
			hasSet = true
		}
		if d.Type == DiffBreakpointCleared && d.Node == "recall" {
			hasCleared = true
		}
	}
	if !hasSet || !hasCleared {
		t.Errorf("expected set and cleared breakpoint diffs, got %+v", diffs)
	}
}

func TestCircuitStore_SetPaused(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.SetPaused(true)
	if !store.Snapshot().Paused {
		t.Error("expected paused")
	}

	store.SetPaused(true) // idempotent
	store.SetPaused(false)
	if store.Snapshot().Paused {
		t.Error("expected unpaused")
	}

	diffs := collectDiffs(ch, 50*time.Millisecond)
	var pauseCount, resumeCount int
	for _, d := range diffs {
		if d.Type == DiffPaused {
			pauseCount++
		}
		if d.Type == DiffResumed {
			resumeCount++
		}
	}
	if pauseCount != 1 {
		t.Errorf("paused diffs = %d, want 1 (idempotent)", pauseCount)
	}
	if resumeCount != 1 {
		t.Errorf("resumed diffs = %d, want 1", resumeCount)
	}
}

func TestCircuitStore_SnapshotIsolation(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	snap1 := store.Snapshot()

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
	snap2 := store.Snapshot()

	if snap1.Nodes["recall"].State != NodeIdle {
		t.Error("snap1 should not be affected by subsequent events")
	}
	if snap2.Nodes["recall"].State != NodeActive {
		t.Error("snap2 should reflect node_enter")
	}
}

func TestCircuitStore_SubscribeUnsubscribe(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	id, ch := store.Subscribe()

	store.Unsubscribe(id)
	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestCircuitStore_Close(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	_, ch := store.Subscribe()

	store.Close()
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after Close()")
	}

	// Double close is safe
	store.Close()
}

func TestCircuitStore_WalkComplete_ClearsWalkers(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	snap := store.Snapshot()
	if len(snap.Walkers) != 1 {
		t.Fatalf("expected 1 walker before complete, got %d", len(snap.Walkers))
	}

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	diffs := collectDiffs(ch, 50*time.Millisecond)

	snap = store.Snapshot()
	if len(snap.Walkers) != 0 {
		t.Errorf("walkers should be empty after walk_complete, got %d: %v",
			len(snap.Walkers), snap.Walkers)
	}

	var hasRemoved bool
	for _, d := range diffs {
		if d.Type == DiffWalkerRemoved && d.Walker == "w1" {
			hasRemoved = true
		}
	}
	if !hasRemoved {
		t.Error("expected walker_removed diff for w1 on walk_complete")
	}
}

func TestCircuitStore_WalkComplete_ClearsMultipleWalkers(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w2"})
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "investigate", Walker: "w3"})

	snap := store.Snapshot()
	if len(snap.Walkers) != 3 {
		t.Fatalf("expected 3 walkers before complete, got %d", len(snap.Walkers))
	}

	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	snap = store.Snapshot()
	if len(snap.Walkers) != 0 {
		t.Errorf("all walkers should be cleared after walk_complete, got %d: %v",
			len(snap.Walkers), snap.Walkers)
	}
}

func TestCircuitStore_CalibrationMultiCase_NoStaleWalkers(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())

	for i := 0; i < 30; i++ {
		walker := circuit.WalkEvent{Walker: "C" + string(rune('0'+i/10)) + string(rune('0'+i%10))}

		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "triage", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "report", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "report", Walker: walker.Walker})
		store.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete, Walker: walker.Walker})
	}

	snap := store.Snapshot()
	if len(snap.Walkers) != 0 {
		t.Errorf("after 30 completed cases, expected 0 stale walkers, got %d", len(snap.Walkers))
		for wid := range snap.Walkers {
			t.Logf("  stale walker: %s", wid)
		}
	}
}

func TestCircuitStore_WalkError_ClearsWalker(t *testing.T) {
	store := NewCircuitStore(testCircuitDef())
	store.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	store.OnEvent(&circuit.WalkEvent{
		Type:   circuit.EventWalkError,
		Node:   "recall",
		Walker: "w1",
		Error:  errors.New("timeout"),
	})

	snap := store.Snapshot()
	if len(snap.Walkers) != 0 {
		t.Errorf("walker should be cleared after walk_error, got %d: %v",
			len(snap.Walkers), snap.Walkers)
	}
}

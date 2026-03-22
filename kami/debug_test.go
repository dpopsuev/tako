package kami

import (
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func TestDebugController_BreakpointPauseResume(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	dc.SetBreakpoint("investigate")

	bps := dc.ListBreakpoints()
	if len(bps) != 1 || bps[0] != "investigate" {
		t.Fatalf("breakpoints = %v, want [investigate]", bps)
	}

	// Simulate walk entering the breakpoint node in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		dc.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeEnter,
			Node:   "investigate",
			Walker: "sentinel",
		})
	}()

	// Drain events until we see breakpoint_hit
	deadline := time.After(2 * time.Second)
	var hitSeen bool
	for !hitSeen {
		select {
		case e := <-ch:
			if e.Type == EventBreakpointHit && e.Node == "investigate" {
				hitSeen = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for breakpoint_hit")
		}
	}

	if dc.State() != StatePaused {
		t.Errorf("state = %v, want paused", dc.State())
	}

	snap := dc.Snapshot()
	if snap.State != "paused" {
		t.Errorf("snapshot state = %q, want paused", snap.State)
	}
	if snap.CurrentNode != "investigate" {
		t.Errorf("snapshot current_node = %q, want investigate", snap.CurrentNode)
	}

	dc.Resume()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnEvent did not resume after Resume()")
	}

	if dc.State() != StateRunning {
		t.Errorf("state after resume = %v, want running", dc.State())
	}
}

func TestDebugController_ManualPause(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	dc.Pause()
	if dc.State() != StatePaused {
		t.Fatalf("state = %v, want paused", dc.State())
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		dc.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeEnter,
			Node:   "triage",
			Walker: "sentinel",
		})
	}()

	time.Sleep(50 * time.Millisecond)
	if dc.State() != StatePaused {
		t.Errorf("state should still be paused")
	}

	dc.Resume()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnEvent did not resume")
	}
}

func TestDebugController_AdvanceNode(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	dc.SetBreakpoint("triage")

	done := make(chan struct{})
	go func() {
		defer close(done)
		dc.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeEnter,
			Node: "triage",
		})
	}()

	time.Sleep(50 * time.Millisecond)
	if dc.State() != StatePaused {
		t.Fatalf("state = %v, want paused", dc.State())
	}

	dc.AdvanceNode()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AdvanceNode did not resume")
	}
}

func TestDebugController_SnapshotTracksVisited(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	nodes := []string{"recall", "triage", "investigate"}
	for _, n := range nodes {
		dc.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeEnter,
			Node: n,
		})
		dc.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeExit,
			Node: n,
		})
	}

	snap := dc.Snapshot()
	if len(snap.NodesVisited) != 3 {
		t.Fatalf("visited = %v, want 3 nodes", snap.NodesVisited)
	}
	for i, want := range nodes {
		if snap.NodesVisited[i] != want {
			t.Errorf("visited[%d] = %q, want %q", i, snap.NodesVisited[i], want)
		}
	}
}

func TestDebugController_Assertions(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	dc.AddAssertion(Assertion{
		Name: "always_passes",
		Predicate: func(_ CircuitSnapshot) error {
			return nil
		},
	})
	dc.AddAssertion(Assertion{
		Name: "always_fails",
		Predicate: func(_ CircuitSnapshot) error {
			return fmt.Errorf("invariant violated")
		},
	})

	errs := dc.RunAssertions()
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
	if errs[0].Error() != `assertion "always_fails": invariant violated` {
		t.Errorf("error = %q", errs[0].Error())
	}
}

func TestDebugController_ClearBreakpoint(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()
	dc := NewDebugController(bridge)

	dc.SetBreakpoint("triage")
	dc.SetBreakpoint("investigate")
	if len(dc.ListBreakpoints()) != 2 {
		t.Fatalf("expected 2 breakpoints")
	}

	dc.ClearBreakpoint("triage")
	bps := dc.ListBreakpoints()
	if len(bps) != 1 || bps[0] != "investigate" {
		t.Fatalf("breakpoints after clear = %v, want [investigate]", bps)
	}
}

func TestDebugController_ImplementsWalkObserver(t *testing.T) {
	var _ circuit.WalkObserver = (*DebugController)(nil)
}

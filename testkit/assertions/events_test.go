package assertions_test

import (
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/testkit/assertions"
)

func TestAssertEventOrder_Pass(t *testing.T) {
	events := []framework.WalkEvent{
		{Type: framework.EventNodeEnter, Node: "A"},
		{Type: framework.EventNodeExit, Node: "A"},
		{Type: framework.EventTransition, Edge: "A-B"},
		{Type: framework.EventNodeEnter, Node: "B"},
		{Type: framework.EventNodeExit, Node: "B"},
	}

	assertions.AssertEventOrder(t, events, []framework.WalkEventType{
		framework.EventNodeEnter,
		framework.EventNodeExit,
		framework.EventNodeEnter,
	})
}

func TestAssertEventOrder_Subsequence(t *testing.T) {
	events := []framework.WalkEvent{
		{Type: framework.EventNodeEnter, Node: "A"},
		{Type: framework.EventEdgeEvaluate},
		{Type: framework.EventTransition},
		{Type: framework.EventNodeEnter, Node: "B"},
	}

	// Should find node_enter -> node_enter even with intervening events.
	assertions.AssertEventOrder(t, events, []framework.WalkEventType{
		framework.EventNodeEnter,
		framework.EventNodeEnter,
	})
}

func TestAssertEventOrder_EmptyExpected(t *testing.T) {
	events := []framework.WalkEvent{
		{Type: framework.EventNodeEnter, Node: "A"},
	}
	// Empty expected should always pass.
	assertions.AssertEventOrder(t, events, nil)
}

func TestAssertNoEvent_Pass(t *testing.T) {
	events := []framework.WalkEvent{
		{Type: framework.EventNodeEnter, Node: "A"},
		{Type: framework.EventNodeExit, Node: "A"},
	}
	assertions.AssertNoEvent(t, events, framework.EventWalkError)
}

func TestAssertNoEvent_EmptyEvents(t *testing.T) {
	assertions.AssertNoEvent(t, nil, framework.EventWalkError)
}

func TestWaitForSignal_Found(t *testing.T) {
	bus := dispatch.NewSignalBus()
	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Emit("target", "agent", "case", "step", nil)
	}()

	assertions.WaitForSignal(t, bus, "target", 2*time.Second)
}

func TestWaitForSignal_AlreadyPresent(t *testing.T) {
	bus := dispatch.NewSignalBus()
	bus.Emit("target", "agent", "case", "step", nil)

	assertions.WaitForSignal(t, bus, "target", 100*time.Millisecond)
}

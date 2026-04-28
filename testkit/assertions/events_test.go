package assertions_test

import (
	"testing"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/testkit/assertions"
	"github.com/dpopsuev/tangle/signal"
)

func TestAssertEventOrder_Pass(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "A"},
		{Type: circuit.EventNodeExit, Node: "A"},
		{Type: circuit.EventTransition, Edge: "A-B"},
		{Type: circuit.EventNodeEnter, Node: "B"},
		{Type: circuit.EventNodeExit, Node: "B"},
	}

	assertions.AssertEventOrder(t, events, []circuit.WalkEventType{
		circuit.EventNodeEnter,
		circuit.EventNodeExit,
		circuit.EventNodeEnter,
	})
}

func TestAssertEventOrder_Subsequence(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "A"},
		{Type: circuit.EventEdgeEvaluate},
		{Type: circuit.EventTransition},
		{Type: circuit.EventNodeEnter, Node: "B"},
	}

	// Should find node_enter -> node_enter even with intervening events.
	assertions.AssertEventOrder(t, events, []circuit.WalkEventType{
		circuit.EventNodeEnter,
		circuit.EventNodeEnter,
	})
}

func TestAssertEventOrder_EmptyExpected(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "A"},
	}
	// Empty expected should always pass.
	assertions.AssertEventOrder(t, events, nil)
}

func TestAssertNoEvent_Pass(t *testing.T) {
	events := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "A"},
		{Type: circuit.EventNodeExit, Node: "A"},
	}
	assertions.AssertNoEvent(t, events, circuit.EventWalkError)
}

func TestAssertNoEvent_EmptyEvents(t *testing.T) {
	assertions.AssertNoEvent(t, nil, circuit.EventWalkError)
}

func TestWaitForSignal_Found(t *testing.T) {
	bus := signal.NewMemBus()
	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Emit(&signal.Signal{Event: "target", Agent: "agent", CaseID: "case", Step: "step"})
	}()

	assertions.WaitForSignal(t, bus, "target", 2*time.Second)
}

func TestWaitForSignal_AlreadyPresent(t *testing.T) {
	bus := signal.NewMemBus()
	bus.Emit(&signal.Signal{Event: "target", Agent: "agent", CaseID: "case", Step: "step"})

	assertions.WaitForSignal(t, bus, "target", 100*time.Millisecond)
}

package kami

import (
	"testing"
	"time"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/circuit"
)

func TestEventBridge_WalkEventMapping(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	bridge.OnEvent(circuit.WalkEvent{
		Type:    circuit.EventNodeEnter,
		Node:    "triage",
		Walker:  "sentinel",
		Elapsed: 150 * time.Millisecond,
	})

	select {
	case e := <-ch:
		if e.Type != EventNodeEnter {
			t.Errorf("Type = %q, want %q", e.Type, EventNodeEnter)
		}
		if e.Node != "triage" {
			t.Errorf("Node = %q, want triage", e.Node)
		}
		if e.Agent != "sentinel" {
			t.Errorf("Agent = %q, want sentinel", e.Agent)
		}
		if e.ElapsedMs != 150 {
			t.Errorf("ElapsedMs = %d, want 150", e.ElapsedMs)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBridge_WalkEventError(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	bridge.OnEvent(circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Node:  "investigate",
		Error: errTest,
	})

	select {
	case e := <-ch:
		if e.Error != "test error" {
			t.Errorf("Error = %q, want 'test error'", e.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestEventBridge_SignalMapping(t *testing.T) {
	bus := signal.NewMemBus()
	bridge := NewEventBridge(bus)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	bus.Emit(&signal.Signal{Event: "step_ready", Agent: "worker-1", CaseID: "case-42", Step: "F3_INVESTIGATE", Meta: map[string]string{"model": "gpt-4"}})

	bridge.StartPolling(10 * time.Millisecond)

	select {
	case e := <-ch:
		if e.Type != EventSignal {
			t.Errorf("Type = %q, want %q", e.Type, EventSignal)
		}
		if e.Agent != "worker-1" {
			t.Errorf("Agent = %q, want worker-1", e.Agent)
		}
		if e.CaseID != "case-42" {
			t.Errorf("CaseID = %q, want case-42", e.CaseID)
		}
		if e.Data["signal_event"] != "step_ready" {
			t.Errorf("Data[signal_event] = %v, want step_ready", e.Data["signal_event"])
		}
		if e.Data["step"] != "F3_INVESTIGATE" {
			t.Errorf("Data[step] = %v, want F3_INVESTIGATE", e.Data["step"])
		}
		if e.Data["model"] != "gpt-4" {
			t.Errorf("Data[model] = %v, want gpt-4", e.Data["model"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signal event")
	}
}

func TestEventBridge_MultipleSubscribers(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id1, ch1 := bridge.Subscribe()
	defer bridge.Unsubscribe(id1)
	id2, ch2 := bridge.Subscribe()
	defer bridge.Unsubscribe(id2)
	id3, ch3 := bridge.Subscribe()
	defer bridge.Unsubscribe(id3)

	bridge.Emit(Event{Type: EventNodeEnter, Node: "test"})

	for i, ch := range []<-chan Event{ch1, ch2, ch3} {
		select {
		case e := <-ch:
			if e.Node != "test" {
				t.Errorf("subscriber %d: Node = %q, want test", i, e.Node)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timeout", i)
		}
	}
}

func TestEventBridge_UnsubscribeStopsDelivery(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	bridge.Unsubscribe(id)

	bridge.Emit(Event{Type: EventNodeEnter, Node: "test"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("received event after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEventBridge_CloseStopsPolling(t *testing.T) {
	bus := signal.NewMemBus()
	bridge := NewEventBridge(bus)
	bridge.StartPolling(10 * time.Millisecond)

	bridge.Close()

	bus.Emit(&signal.Signal{Event: "late", Agent: "agent"})
	time.Sleep(50 * time.Millisecond)
}

func TestEventBridge_ImplementsWalkObserver(t *testing.T) {
	var _ circuit.WalkObserver = (*EventBridge)(nil)
}

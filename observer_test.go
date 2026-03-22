package framework

import (
	"errors"
	"testing"
	"time"
)

func TestTraceCollector_CollectsEvents(t *testing.T) {
	tc := &TraceCollector{}

	tc.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "A", Walker: "herald"})
	tc.OnEvent(WalkEvent{Type: EventNodeExit, Node: "A", Walker: "herald", Elapsed: 5 * time.Millisecond})
	tc.OnEvent(WalkEvent{Type: EventTransition, Node: "A", Edge: "E1"})

	events := tc.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Type != EventNodeEnter {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, EventNodeEnter)
	}
	if events[1].Elapsed != 5*time.Millisecond {
		t.Errorf("events[1].Elapsed = %v, want 5ms", events[1].Elapsed)
	}
	if events[2].Edge != "E1" {
		t.Errorf("events[2].Edge = %q, want %q", events[2].Edge, "E1")
	}
}

func TestTraceCollector_EventsOfType(t *testing.T) {
	tc := &TraceCollector{}
	tc.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "A"})
	tc.OnEvent(WalkEvent{Type: EventTransition, Node: "A", Edge: "E1"})
	tc.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "B"})
	tc.OnEvent(WalkEvent{Type: EventWalkComplete})

	enters := tc.EventsOfType(EventNodeEnter)
	if len(enters) != 2 {
		t.Fatalf("expected 2 node_enter events, got %d", len(enters))
	}
	if enters[0].Node != "A" || enters[1].Node != "B" {
		t.Errorf("unexpected nodes: %v, %v", enters[0].Node, enters[1].Node)
	}
}

func TestTraceCollector_Reset(t *testing.T) {
	tc := &TraceCollector{}
	tc.OnEvent(WalkEvent{Type: EventNodeEnter})
	tc.OnEvent(WalkEvent{Type: EventNodeEnter})

	if len(tc.Events()) != 2 {
		t.Fatalf("expected 2 events before reset")
	}

	tc.Reset()
	if len(tc.Events()) != 0 {
		t.Errorf("expected 0 events after reset, got %d", len(tc.Events()))
	}
}

func TestTraceCollector_EventsReturnsCopy(t *testing.T) {
	tc := &TraceCollector{}
	tc.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "A"})

	events := tc.Events()
	events[0].Node = "mutated"

	original := tc.Events()
	if original[0].Node != "A" {
		t.Error("Events() did not return a copy — mutation leaked")
	}
}

func TestWalkObserverFunc(t *testing.T) {
	var received WalkEvent
	fn := WalkObserverFunc(func(e WalkEvent) {
		received = e
	})

	fn.OnEvent(WalkEvent{Type: EventWalkError, Error: errors.New("test")})
	if received.Type != EventWalkError {
		t.Errorf("expected EventWalkError, got %q", received.Type)
	}
	if received.Error == nil || received.Error.Error() != "test" {
		t.Errorf("expected error 'test', got %v", received.Error)
	}
}

func TestMultiObserver(t *testing.T) {
	tc1 := &TraceCollector{}
	tc2 := &TraceCollector{}

	multi := MultiObserver{tc1, tc2}
	multi.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "X"})

	if len(tc1.Events()) != 1 {
		t.Errorf("tc1 expected 1 event, got %d", len(tc1.Events()))
	}
	if len(tc2.Events()) != 1 {
		t.Errorf("tc2 expected 1 event, got %d", len(tc2.Events()))
	}
}

func TestLogObserver_NilLogger(t *testing.T) {
	obs := NewLogObserver(nil)
	obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "A", Walker: "herald"})
	obs.OnEvent(WalkEvent{Type: EventWalkError, Error: errors.New("boom")})
}

func TestEmitEvent_NilObserver(t *testing.T) {
	emitEvent(nil, WalkEvent{Type: EventNodeEnter})
}

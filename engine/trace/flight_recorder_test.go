package trace

import (
	"errors"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

func TestFlightRecorder_Record_And_Events(t *testing.T) {
	r := NewFlightRecorder(100)
	r.Record("session:start", "in", "scenario=ptp", nil, nil)
	r.Record("walker:enter", "in", "node=triage", nil, nil)
	r.Record("walker:exit", "out", "node=triage", nil, nil)

	events := r.Events()
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].Station != "session:start" {
		t.Errorf("events[0].Station = %q, want session:start", events[0].Station)
	}
	if events[1].Dir != "in" {
		t.Errorf("events[1].Dir = %q, want in", events[1].Dir)
	}
}

func TestFlightRecorder_RingBuffer_Wraps(t *testing.T) {
	r := NewFlightRecorder(3) // capacity 3
	r.Record("a", "in", "1", nil, nil)
	r.Record("b", "in", "2", nil, nil)
	r.Record("c", "in", "3", nil, nil)
	r.Record("d", "in", "4", nil, nil) // overwrites "a"

	events := r.Events()
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (ring buffer)", len(events))
	}
	// Oldest surviving is "b" (summary="2"), newest is "d" (summary="4")
	if events[0].Summary != "2" {
		t.Errorf("oldest event summary = %q, want 2", events[0].Summary)
	}
	if events[2].Summary != "4" {
		t.Errorf("newest event summary = %q, want 4", events[2].Summary)
	}
}

func TestFlightRecorder_Query(t *testing.T) {
	r := NewFlightRecorder(100)
	r.Record("walker:enter", "in", "node=a", nil, nil)
	r.Record("edge:evaluate", "in", "edge=e1", nil, nil)
	r.Record("walker:enter", "in", "node=b", nil, nil)
	r.Record("edge:evaluate", "in", "edge=e2", nil, nil)

	walkerEvents := r.Query("walker:")
	if len(walkerEvents) != 2 {
		t.Errorf("got %d walker events, want 2", len(walkerEvents))
	}
	edgeEvents := r.Query("edge:")
	if len(edgeEvents) != 2 {
		t.Errorf("got %d edge events, want 2", len(edgeEvents))
	}
}

func TestFlightRecorder_OnEvent_WalkObserver(t *testing.T) {
	r := NewFlightRecorder(100)

	// Verify it implements WalkObserver.
	var obs circuit.WalkObserver = r
	obs.OnEvent(&circuit.WalkEvent{
		Type:   circuit.EventNodeEnter,
		Node:   "triage",
		Walker: "w1",
	})
	obs.OnEvent(&circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Node:  "triage",
		Error: errors.New("boom"),
	})

	events := r.Events()
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Station != "walker:enter" {
		t.Errorf("station = %q, want walker:enter", events[0].Station)
	}
	if events[1].Station != "circuit:error" {
		t.Errorf("station = %q, want circuit:error", events[1].Station)
	}
	if events[1].Err == nil {
		t.Error("expected error on circuit:error event")
	}
}

func TestFlightRecorder_Dump(t *testing.T) {
	r := NewFlightRecorder(100)
	r.Record("session:start", "in", "test", nil, nil)
	r.Record("circuit:error", "out", "boom", nil, errors.New("something failed"))

	// Dump should not panic. Visual output goes to test log.
	r.Dump(t)
}

func TestFlightRecorder_Reset(t *testing.T) {
	r := NewFlightRecorder(100)
	r.Record("a", "in", "1", nil, nil)
	r.Reset()

	if len(r.Events()) != 0 {
		t.Error("expected empty after Reset")
	}
}

func TestFlightRecorder_DefaultCapacity(t *testing.T) {
	r := NewFlightRecorder(0) // should default to 1000
	if r.cap != 1000 {
		t.Errorf("cap = %d, want 1000", r.cap)
	}
}

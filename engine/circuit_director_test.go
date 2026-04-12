package engine

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe"
)

func TestCircuitDetail_String_WithArtifact(t *testing.T) {
	d := CircuitDetail{Node: "triage", Artifact: "classification", Confidence: 0.85}
	s := d.String()
	if s != "node=triage artifact=classification conf=0.85" {
		t.Errorf("String() = %q", s)
	}
}

func TestCircuitDetail_String_WithoutArtifact(t *testing.T) {
	d := CircuitDetail{Node: "triage"}
	s := d.String()
	if s != "node=triage" {
		t.Errorf("String() = %q", s)
	}
}

func TestChannelObserver_NodeEnter(t *testing.T) {
	ch := make(chan troupe.Event, 10) //nolint:mnd // buffer
	obs := &channelObserver{ch: ch}

	obs.OnEvent(&circuit.WalkEvent{
		Type:   circuit.EventNodeEnter,
		Node:   "triage",
		Walker: "sentinel",
	})

	select {
	case ev := <-ch:
		if ev.Kind != eventStarted {
			t.Errorf("Kind = %s, want started", ev.Kind)
		}
		if ev.Step != "triage" {
			t.Errorf("Step = %q, want triage", ev.Step)
		}
		if ev.Agent != "sentinel" {
			t.Errorf("Agent = %q, want sentinel", ev.Agent)
		}
	default:
		t.Fatal("no event received")
	}
}

func TestChannelObserver_NodeExit_Success(t *testing.T) {
	ch := make(chan troupe.Event, 10) //nolint:mnd // buffer
	obs := &channelObserver{ch: ch}

	obs.OnEvent(&circuit.WalkEvent{
		Type:    circuit.EventNodeExit,
		Node:    "investigate",
		Walker:  "seeker",
		Elapsed: 500 * time.Millisecond, //nolint:mnd // test value
	})

	ev := <-ch
	if ev.Kind != eventCompleted {
		t.Errorf("Kind = %s, want completed", ev.Kind)
	}
	if ev.Elapsed != 500*time.Millisecond { //nolint:mnd // test value
		t.Errorf("Elapsed = %v", ev.Elapsed)
	}
}

func TestChannelObserver_NodeExit_Error(t *testing.T) {
	ch := make(chan troupe.Event, 10) //nolint:mnd // buffer
	obs := &channelObserver{ch: ch}

	obs.OnEvent(&circuit.WalkEvent{
		Type:  circuit.EventNodeExit,
		Node:  "investigate",
		Error: errTestFailed,
	})

	ev := <-ch
	if ev.Kind != eventFailed {
		t.Errorf("Kind = %s, want failed", ev.Kind)
	}
	if ev.Error == nil {
		t.Error("expected non-nil error")
	}
}

func TestChannelObserver_Transition(t *testing.T) {
	ch := make(chan troupe.Event, 10) //nolint:mnd // buffer
	obs := &channelObserver{ch: ch}

	obs.OnEvent(&circuit.WalkEvent{
		Type: circuit.EventTransition,
		Node: "triage",
		Edge: "triage-investigate",
	})

	ev := <-ch
	if ev.Kind != eventTransition {
		t.Errorf("Kind = %s, want transition", ev.Kind)
	}
	detail, ok := ev.Detail.(CircuitDetail)
	if !ok {
		t.Fatal("Detail is not CircuitDetail")
	}
	if detail.Edge != "triage-investigate" {
		t.Errorf("Edge = %q", detail.Edge)
	}
}

func TestChannelObserver_SkipsUnknownEvents(t *testing.T) {
	ch := make(chan troupe.Event, 10) //nolint:mnd // buffer
	obs := &channelObserver{ch: ch}

	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	select {
	case <-ch:
		t.Error("should not emit event for WalkComplete")
	default:
		// expected
	}
}

func TestChannelObserver_DropsWhenFull(t *testing.T) {
	ch := make(chan troupe.Event) // unbuffered — always full
	obs := &channelObserver{ch: ch}

	// Should not block or panic
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "test"})
}

var errTestFailed = circuit.ErrNodeNotFound // reuse existing sentinel

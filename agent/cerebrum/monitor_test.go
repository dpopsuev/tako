package cerebrum

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestMonitor_ClassifiesEvents(t *testing.T) {
	completer := &stubCompleter{response: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer, WithMaxTurns(3))

	m := reactivity.NewMolecule("test-mol")
	cb.molecule = m

	monitorBus := NewChannelBus(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go cb.Monitor(ctx, monitorBus)

	monitorBus.Send(ctx, Event{
		ID:   "e1",
		Kind: "sensory.timer",
		Payload: []byte("timer fired"),
	})

	time.Sleep(50 * time.Millisecond)

	parked := cb.DrainMonitorEvents()
	// sensory.timer → PriorityInterrupt → injected directly, NOT parked
	// but with default classifier, sensory.timer is Interrupt
	// so it should be injected as atom, not parked

	// Check that the atom was injected into the molecule
	atoms := m.Atoms(reactivity.IntentAtom)
	if len(atoms) == 0 && len(parked) == 0 {
		t.Error("event should have been either injected or parked")
	}
	t.Logf("Injected atoms: %d, Parked events: %d", len(atoms), len(parked))
}

func TestMonitor_ParkEvent(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	m := reactivity.NewMolecule("test-mol")
	cb.molecule = m

	// Override classifier to always park
	cb.priorityClassifier = classifierFunc(func(_ Event, _ *reactivity.Molecule) Priority {
		return PriorityPark
	})

	monitorBus := NewChannelBus(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go cb.Monitor(ctx, monitorBus)

	monitorBus.Send(ctx, Event{ID: "e1", Kind: "test.event"})
	monitorBus.Send(ctx, Event{ID: "e2", Kind: "test.event"})

	time.Sleep(50 * time.Millisecond)
	cancel()

	parked := cb.DrainMonitorEvents()
	if len(parked) != 2 {
		t.Errorf("expected 2 parked events, got %d", len(parked))
	}
}

func TestMonitor_IgnoreEvent(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	m := reactivity.NewMolecule("test-mol")
	cb.molecule = m

	cb.priorityClassifier = classifierFunc(func(_ Event, _ *reactivity.Molecule) Priority {
		return PriorityIgnore
	})

	monitorBus := NewChannelBus(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go cb.Monitor(ctx, monitorBus)

	monitorBus.Send(ctx, Event{ID: "e1", Kind: "noise"})

	time.Sleep(50 * time.Millisecond)
	cancel()

	parked := cb.DrainMonitorEvents()
	if len(parked) != 0 {
		t.Errorf("ignored events should not be parked, got %d", len(parked))
	}
}

func TestDrainMonitorEvents_EmptyWhenNone(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	events := cb.DrainMonitorEvents()
	if len(events) != 0 {
		t.Errorf("expected 0, got %d", len(events))
	}
}

type classifierFunc func(Event, *reactivity.Molecule) Priority

func (f classifierFunc) Classify(e Event, m *reactivity.Molecule) Priority {
	return f(e, m)
}

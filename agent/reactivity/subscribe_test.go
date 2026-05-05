package reactivity

import (
	"fmt"
	"testing"
	"time"
)

func TestMolecule_Subscribe_InsertAtom(t *testing.T) {
	m := NewMolecule("test")
	var events []MoleculeEvent
	m.Subscribe(func(e MoleculeEvent) {
		events = append(events, e)
	})

	m.InsertAtom(Atom{
		ID: "a1", Type: IntentAtom, Taxonomy: "intent.test",
		Content: []byte("hello"), CreatedAt: time.Now(),
	})

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != "atom_inserted" {
		t.Errorf("expected atom_inserted, got %s", events[0].Kind)
	}
	if events[0].Atom == nil {
		t.Error("atom should be non-nil for atom_inserted")
	}
	if events[0].MoleculeID != "test" {
		t.Errorf("expected molecule ID 'test', got %s", events[0].MoleculeID)
	}
}

func TestMolecule_Subscribe_SetPhase_FiresOnChange(t *testing.T) {
	m := NewMolecule("test")
	var events []MoleculeEvent
	m.Subscribe(func(e MoleculeEvent) {
		events = append(events, e)
	})

	m.SetPhase(AssessmentAtom)

	if len(events) != 1 {
		t.Fatalf("expected 1 event on phase change, got %d", len(events))
	}
	if events[0].Kind != "phase_changed" {
		t.Errorf("expected phase_changed, got %s", events[0].Kind)
	}
	if events[0].Phase != AssessmentAtom {
		t.Errorf("expected assessment phase, got %s", events[0].Phase)
	}

	m.SetPhase(AssessmentAtom)
	if len(events) != 1 {
		t.Errorf("expected no event on same-phase set, got %d events", len(events))
	}
}

func TestMolecule_Subscribe_Seal(t *testing.T) {
	m := NewMolecule("test")
	var events []MoleculeEvent
	m.Subscribe(func(e MoleculeEvent) {
		events = append(events, e)
	})

	m.Seal(Atom{ID: "wish", Taxonomy: "retrospection.test", CreatedAt: time.Now()})

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != "sealed" {
		t.Errorf("expected sealed, got %s", events[0].Kind)
	}
}

func TestMolecule_Subscribe_NoSubscribers_NoPanic(t *testing.T) {
	m := NewMolecule("test")

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})
	m.SetPhase(AssessmentAtom)
	m.Seal(Atom{ID: "wish", CreatedAt: time.Now()})
}

func TestMolecule_Subscribe_PanickingSubscriber_Recovers(t *testing.T) {
	m := NewMolecule("test")
	var secondCalled bool

	m.Subscribe(func(e MoleculeEvent) {
		panic("boom")
	})
	m.Subscribe(func(e MoleculeEvent) {
		secondCalled = true
	})

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})

	if !secondCalled {
		t.Error("second subscriber should still be called after first panics")
	}
}

func TestMolecule_Subscribe_MultipleSubscribers(t *testing.T) {
	m := NewMolecule("test")
	var count1, count2 int

	m.Subscribe(func(e MoleculeEvent) { count1++ })
	m.Subscribe(func(e MoleculeEvent) { count2++ })

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})
	m.SetPhase(AssessmentAtom)

	if count1 != 2 {
		t.Errorf("subscriber 1: expected 2 events, got %d", count1)
	}
	if count2 != 2 {
		t.Errorf("subscriber 2: expected 2 events, got %d", count2)
	}
}

func TestMolecule_On_TypedChannel(t *testing.T) {
	m := NewMolecule("test")
	var sealEvents []MoleculeEvent

	m.On("sealed", &testListener{name: "seal-watcher", fn: func(e MoleculeEvent) {
		sealEvents = append(sealEvents, e)
	}})

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})
	m.SetPhase(AssessmentAtom)
	m.Seal(Atom{ID: "wish", CreatedAt: time.Now()})

	if len(sealEvents) != 1 {
		t.Fatalf("expected 1 sealed event, got %d", len(sealEvents))
	}
	if sealEvents[0].Kind != "sealed" {
		t.Errorf("expected sealed, got %s", sealEvents[0].Kind)
	}
}

func TestMolecule_On_MultipleKinds(t *testing.T) {
	m := NewMolecule("test")
	var atomEvents, phaseEvents int

	m.On("atom_inserted", &testListener{name: "atom-counter", fn: func(e MoleculeEvent) { atomEvents++ }})
	m.On("phase_changed", &testListener{name: "phase-counter", fn: func(e MoleculeEvent) { phaseEvents++ }})

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})
	m.InsertAtom(Atom{ID: "a2", Type: AssessmentAtom, CreatedAt: time.Now()})
	m.SetPhase(AssessmentAtom)

	if atomEvents != 2 {
		t.Errorf("atom events: expected 2, got %d", atomEvents)
	}
	if phaseEvents != 1 {
		t.Errorf("phase events: expected 1, got %d", phaseEvents)
	}
}

func TestMolecule_On_NamedListener_PanicLogsName(t *testing.T) {
	m := NewMolecule("test")
	var secondCalled bool

	m.On("atom_inserted", &testListener{name: "crasher", fn: func(e MoleculeEvent) {
		panic("boom")
	}})
	m.On("atom_inserted", &testListener{name: "survivor", fn: func(e MoleculeEvent) {
		secondCalled = true
	}})

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})

	if !secondCalled {
		t.Error("survivor listener should still be called after crasher panics")
	}
}

func TestMolecule_EventLog(t *testing.T) {
	m := NewMolecule("test")

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, CreatedAt: time.Now()})
	m.SetPhase(AssessmentAtom)
	m.Seal(Atom{ID: "wish", CreatedAt: time.Now()})

	log := m.EventLog()
	if len(log) != 3 {
		t.Fatalf("expected 3 events in log, got %d", len(log))
	}
	if log[0].Kind != "atom_inserted" {
		t.Errorf("log[0]: expected atom_inserted, got %s", log[0].Kind)
	}
	if log[1].Kind != "phase_changed" {
		t.Errorf("log[1]: expected phase_changed, got %s", log[1].Kind)
	}
	if log[2].Kind != "sealed" {
		t.Errorf("log[2]: expected sealed, got %s", log[2].Kind)
	}
}

func TestMolecule_EventLog_RingBuffer(t *testing.T) {
	m := NewMolecule("test")
	m.eventLogCap = 3

	for i := 0; i < 10; i++ {
		m.InsertAtom(Atom{ID: fmt.Sprintf("a%d", i), Type: IntentAtom, CreatedAt: time.Now()})
	}

	log := m.EventLog()
	if len(log) != 3 {
		t.Fatalf("ring buffer should cap at 3, got %d", len(log))
	}
}

type testListener struct {
	name string
	fn   func(MoleculeEvent)
}

func (l *testListener) Name() string                  { return l.name }
func (l *testListener) OnMoleculeEvent(e MoleculeEvent) { l.fn(e) }

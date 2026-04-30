package cerebrum

import (
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestStore_ReceiveAtom_LandsInUnsorted(t *testing.T) {
	s := NewMoleculeStore()
	atom := reactivity.Atom{
		ID:        "a1",
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.goal.test",
		Content:   []byte("test"),
		CreatedAt: time.Now(),
	}

	s.Receive(atom)

	unsorted := s.Unsorted()
	if len(unsorted) != 1 {
		t.Fatalf("expected 1 unsorted atom, got %d", len(unsorted))
	}
	if unsorted[0].ID != "a1" {
		t.Errorf("expected atom a1, got %s", unsorted[0].ID)
	}
}

func TestStore_Focus_CreatesMoleculeIfMissing(t *testing.T) {
	s := NewMoleculeStore()

	m := s.Focus("mol-1")
	if m == nil {
		t.Fatal("Focus should create molecule if missing")
	}
	if m.ID != "mol-1" {
		t.Errorf("expected mol-1, got %s", m.ID)
	}

	m2 := s.Focus("mol-1")
	if m2 != m {
		t.Error("Focus should return same molecule for same ID")
	}
}

func TestStore_Park_DeactivatesFocused(t *testing.T) {
	s := NewMoleculeStore()

	s.Focus("mol-1")
	if s.FocusedID() != "mol-1" {
		t.Fatalf("expected focused mol-1, got %s", s.FocusedID())
	}

	s.Park()
	if s.FocusedID() != "" {
		t.Errorf("expected no focused molecule after Park, got %s", s.FocusedID())
	}
}

func TestStore_FocusSwitch(t *testing.T) {
	s := NewMoleculeStore()

	m1 := s.Focus("mol-1")
	m2 := s.Focus("mol-2")

	if s.FocusedID() != "mol-2" {
		t.Errorf("expected mol-2 focused, got %s", s.FocusedID())
	}

	if m1 == m2 {
		t.Error("different molecules should be different pointers")
	}

	m1Again := s.Focus("mol-1")
	if m1Again != m1 {
		t.Error("re-focusing mol-1 should return original molecule")
	}
}

func TestStore_Molecules_ListsAll(t *testing.T) {
	s := NewMoleculeStore()

	s.Focus("mol-a")
	s.Focus("mol-b")
	s.Focus("mol-c")

	ids := s.Molecules()
	if len(ids) != 3 {
		t.Fatalf("expected 3 molecules, got %d", len(ids))
	}
}

func TestStore_Drain_RemovesUnsorted(t *testing.T) {
	s := NewMoleculeStore()
	s.Receive(reactivity.Atom{ID: "a1", Type: reactivity.IntentAtom, CreatedAt: time.Now()})
	s.Receive(reactivity.Atom{ID: "a2", Type: reactivity.IntentAtom, CreatedAt: time.Now()})

	drained := s.Drain()
	if len(drained) != 2 {
		t.Fatalf("expected 2 drained atoms, got %d", len(drained))
	}

	if len(s.Unsorted()) != 0 {
		t.Error("unsorted should be empty after drain")
	}
}

func TestStore_Focused_ReturnsNilWhenNone(t *testing.T) {
	s := NewMoleculeStore()

	m := s.Focused()
	if m != nil {
		t.Error("Focused should return nil when no molecule is focused")
	}
}

func TestStore_Park_NoOpWhenNothingFocused(t *testing.T) {
	s := NewMoleculeStore()
	s.Park()
}

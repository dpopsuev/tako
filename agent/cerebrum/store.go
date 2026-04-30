package cerebrum

import (
	"sync"

	"github.com/dpopsuev/tako/agent/reactivity"
)

// MoleculeStore is the Cerebrum's internal Depo — Molecules are stored,
// focused, and parked using Depo semantics.
//
// Unsorted: raw atoms arrive here before triage assigns them to a Molecule.
// Sorted:   per-Molecule shelves, one is "focused" (active work).
// Focus:    pull a Molecule shelf into active processing.
// Park:     push the focused Molecule back to storage.
type MoleculeStore struct {
	mu        sync.Mutex
	unsorted  []reactivity.Atom
	molecules map[string]*reactivity.Molecule
	focusedID string
}

func NewMoleculeStore() *MoleculeStore {
	return &MoleculeStore{
		molecules: make(map[string]*reactivity.Molecule),
	}
}

func (s *MoleculeStore) Receive(atom reactivity.Atom) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unsorted = append(s.unsorted, atom)
}

func (s *MoleculeStore) Unsorted() []reactivity.Atom {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]reactivity.Atom(nil), s.unsorted...)
}

func (s *MoleculeStore) Drain() []reactivity.Atom {
	s.mu.Lock()
	defer s.mu.Unlock()
	drained := s.unsorted
	s.unsorted = nil
	return drained
}

func (s *MoleculeStore) Focus(moleculeID string) *reactivity.Molecule {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.molecules[moleculeID]
	if !ok {
		m = reactivity.NewMolecule(moleculeID)
		s.molecules[moleculeID] = m
	}
	s.focusedID = moleculeID
	return m
}

func (s *MoleculeStore) Focused() *reactivity.Molecule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.focusedID == "" {
		return nil
	}
	return s.molecules[s.focusedID]
}

func (s *MoleculeStore) FocusedID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.focusedID
}

func (s *MoleculeStore) Park() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.focusedID = ""
}

func (s *MoleculeStore) Molecules() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.molecules))
	for id := range s.molecules {
		out = append(out, id)
	}
	return out
}

func (s *MoleculeStore) Molecule(id string) (*reactivity.Molecule, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.molecules[id]
	return m, ok
}

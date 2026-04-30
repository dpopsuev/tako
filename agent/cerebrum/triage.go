package cerebrum

import "github.com/dpopsuev/tako/agent/reactivity"

// TriageVerdict classifies an incoming atom's relationship to existing molecules.
type TriageVerdict int

const (
	Inception     TriageVerdict = iota // new molecule — no existing molecule handles this domain
	Continuation                       // existing molecule — atom extends current work
	Contradiction                      // conflict — atom contradicts an existing atom
)

func (v TriageVerdict) String() string {
	switch v {
	case Inception:
		return "inception"
	case Continuation:
		return "continuation"
	case Contradiction:
		return "contradiction"
	default:
		return "unknown"
	}
}

// TriageResult pairs a verdict with the target molecule ID.
type TriageResult struct {
	Verdict    TriageVerdict
	MoleculeID string
	Conflict   *reactivity.Atom
}

// Triage classifies an atom against the MoleculeStore and Reactor.
// Drains the unsorted shelf, classifies each atom, and adds it to
// the appropriate molecule in the store.
func Triage(store *MoleculeStore, reactor *reactivity.Reactor) []TriageResult {
	atoms := store.Drain()
	results := make([]TriageResult, 0, len(atoms))

	for _, atom := range atoms {
		result := classify(store, reactor, atom)
		m := store.Focus(result.MoleculeID)
		reactor.Add(m, atom)
		store.Park()
		results = append(results, result)
	}

	return results
}

func classify(store *MoleculeStore, reactor *reactivity.Reactor, atom reactivity.Atom) TriageResult {
	for _, id := range store.Molecules() {
		m, ok := store.Molecule(id)
		if !ok || m.Sealed() {
			continue
		}

		if !matchesDomain(m, atom) {
			continue
		}

		contradicts, conflicting := sameTypeConflict(m, atom)
		if contradicts {
			return TriageResult{
				Verdict:    Contradiction,
				MoleculeID: id,
				Conflict:   conflicting,
			}
		}

		return TriageResult{
			Verdict:    Continuation,
			MoleculeID: id,
		}
	}

	return TriageResult{
		Verdict:    Inception,
		MoleculeID: atom.ID + "-mol",
	}
}

func sameTypeConflict(m *reactivity.Molecule, atom reactivity.Atom) (bool, *reactivity.Atom) {
	for _, existing := range m.Atoms(atom.Type) {
		if existing.ID != atom.ID && taxonomyDomain(existing.Taxonomy) == taxonomyDomain(atom.Taxonomy) {
			return true, existing
		}
	}
	return false, nil
}

func matchesDomain(m *reactivity.Molecule, atom reactivity.Atom) bool {
	domain := taxonomyDomain(atom.Taxonomy)
	if domain == "" {
		return false
	}
	for _, phase := range []reactivity.AtomType{
		reactivity.IntentAtom, reactivity.AssessmentAtom, reactivity.PlanAtom,
		reactivity.ExecutionAtom, reactivity.RetrospectionAtom,
	} {
		for _, existing := range m.Atoms(phase) {
			if taxonomyDomain(existing.Taxonomy) == domain {
				return true
			}
		}
	}
	return false
}

func taxonomyDomain(taxonomy string) string {
	for i := len(taxonomy) - 1; i >= 0; i-- {
		if taxonomy[i] == '.' {
			return taxonomy[i+1:]
		}
	}
	return ""
}

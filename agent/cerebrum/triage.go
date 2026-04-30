package cerebrum

import "github.com/dpopsuev/tako/agent/reactivity"

// RouteVerdict classifies an incoming atom's relationship to existing molecules.
type RouteVerdict int

const (
	Inception     RouteVerdict = iota // new molecule — no existing molecule handles this domain
	Continuation                       // existing molecule — atom extends current work
	Contradiction                      // conflict — atom contradicts an existing atom
)

func (v RouteVerdict) String() string {
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

// RouteResult pairs a verdict with the target molecule ID.
type RouteResult struct {
	Verdict    RouteVerdict
	MoleculeID string
	Conflict   *reactivity.Atom
}

// Triage classifies an atom against the MoleculeStore and Reactor.
// Drains the unsorted shelf, classifies each atom, and adds it to
// the appropriate molecule in the store.
func Route(store *MoleculeStore, reactor *reactivity.Core) []RouteResult {
	atoms := store.Drain()
	results := make([]RouteResult, 0, len(atoms))

	for _, atom := range atoms {
		result := classify(store, reactor, atom)
		m := store.Focus(result.MoleculeID)
		reactor.Add(m, atom)
		store.Park()
		results = append(results, result)
	}

	return results
}

func classify(store *MoleculeStore, reactor *reactivity.Core, atom reactivity.Atom) RouteResult {
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
			return RouteResult{
				Verdict:    Contradiction,
				MoleculeID: id,
				Conflict:   conflicting,
			}
		}

		return RouteResult{
			Verdict:    Continuation,
			MoleculeID: id,
		}
	}

	return RouteResult{
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
	for _, phase := range reactivity.AllAtomTypes() {
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

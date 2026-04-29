package reactivity

import (
	"fmt"
	"strings"
	"time"
)

// Circuit is the inner thinking loop — a Fab inside the agent's head.
type Circuit struct {
	id          string
	atoms       map[string]*Atom
	edges       map[string][]string
	subgraphs   map[AtomType][]string
	taxonomy    map[string][]string
	mass        map[AtomType]int
	sourceMass  map[AtomSource]int
	triadSealed map[Triad]bool
	phase       AtomType
	sealed      bool
	createdAt   time.Time
}

// NewCircuit creates a ReActivity Circuit starting at Intent phase.
func NewCircuit(id string) *Circuit {
	return &Circuit{
		id:          id,
		atoms:       make(map[string]*Atom),
		edges:       make(map[string][]string),
		subgraphs:   make(map[AtomType][]string),
		taxonomy:    make(map[string][]string),
		mass:        make(map[AtomType]int),
		sourceMass:  make(map[AtomSource]int),
		triadSealed: make(map[Triad]bool),
		phase:       IntentAtom,
		createdAt:   time.Now(),
	}
}

// Phase returns the current phase.
func (c *Circuit) Phase() AtomType {
	return c.phase
}

// Sealed returns true if the circuit has been sealed by a Wish.
func (c *Circuit) Sealed() bool {
	return c.sealed
}

// Mass returns atom count for a given phase.
func (c *Circuit) Mass(t AtomType) int {
	return c.mass[t]
}

// SourceMass returns atom count by source.
func (c *Circuit) SourceMass(s AtomSource) int {
	return c.sourceMass[s]
}

// CurrentTriad returns which triad the circuit is in.
func (c *Circuit) CurrentTriad() Triad {
	return TriadOf(c.phase)
}

// TriadSealed returns whether a triad is sealed.
func (c *Circuit) TriadSealed(t Triad) bool {
	return c.triadSealed[t]
}

// AllTriadsSealed returns true when Reason, Plan, and Act are all sealed.
func (c *Circuit) AllTriadsSealed() bool {
	return c.triadSealed[ReasonTriad] && c.triadSealed[PlanTriad] && c.triadSealed[ActTriad]
}

// UnsealTriad unseals a triad and all lower triads (cascade down).
// Adapt never unseals Reason (caller enforces — North Star is fixed).
func (c *Circuit) UnsealTriad(t Triad) {
	switch t {
	case ReasonTriad:
		c.triadSealed[ReasonTriad] = false
		c.triadSealed[PlanTriad] = false
		c.triadSealed[ActTriad] = false
	case PlanTriad:
		c.triadSealed[PlanTriad] = false
		c.triadSealed[ActTriad] = false
	case ActTriad:
		c.triadSealed[ActTriad] = false
	}
}

// TotalMass returns total atom count.
func (c *Circuit) TotalMass() int {
	total := 0
	for _, v := range c.mass {
		total += v
	}
	return total
}

// Atoms returns all atoms of a given type.
func (c *Circuit) Atoms(t AtomType) []*Atom {
	ids := c.subgraphs[t]
	out := make([]*Atom, 0, len(ids))
	for _, id := range ids {
		if a, ok := c.atoms[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

// Atom returns a single atom by ID.
func (c *Circuit) Atom(id string) (*Atom, bool) {
	a, ok := c.atoms[id]
	return a, ok
}

// ByTaxonomy returns atoms matching a taxonomy prefix.
func (c *Circuit) ByTaxonomy(prefix string) []*Atom {
	var out []*Atom
	for key, ids := range c.taxonomy {
		if strings.HasPrefix(key, prefix) {
			for _, id := range ids {
				if a, ok := c.atoms[id]; ok {
					out = append(out, a)
				}
			}
		}
	}
	return out
}

// EdgesFrom returns atom IDs that this atom targets.
func (c *Circuit) EdgesFrom(atomID string) []string {
	return c.edges[atomID]
}

// Add inserts an atom into the circuit, creates edges, updates indexes, runs Assert.
func (c *Circuit) Add(atom Atom) (AssertResult, Fortune) {
	if c.sealed {
		return Unresolvable, Fortune{Result: Unresolvable, Message: "circuit is sealed"}
	}

	if atom.Type > c.phase && atom.Type != AssessmentAtom {
		return Incompatible, Fortune{
			Result:  Incompatible,
			Message: fmt.Sprintf("circuit is in %s phase, cannot accept future %s atom", c.phase, atom.Type),
			Phase:   c.phase,
		}
	}

	c.atoms[atom.ID] = &atom
	c.subgraphs[atom.Type] = append(c.subgraphs[atom.Type], atom.ID)
	c.mass[atom.Type]++
	c.sourceMass[atom.Source]++

	if atom.Taxonomy != "" {
		c.taxonomy[atom.Taxonomy] = append(c.taxonomy[atom.Taxonomy], atom.ID)
	}

	c.edges[atom.ID] = append(c.edges[atom.ID], atom.Targets...)

	result, fortune := c.assertPhase()
	if result == Pass {
		c.advancePhase()
	}
	return result, fortune
}

// Seal marks the circuit as complete with a Wish atom.
func (c *Circuit) Seal(wish Atom) {
	wish.Type = RetrospectionAtom
	c.atoms[wish.ID] = &wish
	c.subgraphs[RetrospectionAtom] = append(c.subgraphs[RetrospectionAtom], wish.ID)
	c.mass[RetrospectionAtom]++
	if wish.Taxonomy != "" {
		c.taxonomy[wish.Taxonomy] = append(c.taxonomy[wish.Taxonomy], wish.ID)
	}
	c.sealed = true
}

// Contradict checks if an atom contradicts an existing atom about the same concern.
// Uses taxonomy domain (last segment) to match: "assessment.state.floor-dusty"
// contradicts "execution.result.floor-done" because both concern "floor".
func (c *Circuit) Contradict(atom Atom) (bool, *Atom) {
	domain := taxonomyDomain(atom.Taxonomy)
	if domain == "" {
		return false, nil
	}
	for _, existing := range c.atomsByDomain(domain) {
		if existing.ID != atom.ID && existing.Type != atom.Type {
			return true, existing
		}
	}
	return false, nil
}

func taxonomyDomain(taxonomy string) string {
	parts := strings.Split(taxonomy, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func (c *Circuit) atomsByDomain(domain string) []*Atom {
	var out []*Atom
	for _, a := range c.atoms {
		if taxonomyDomain(a.Taxonomy) == domain {
			out = append(out, a)
		}
	}
	return out
}

func (c *Circuit) assertPhase() (AssertResult, Fortune) {
	switch c.phase {
	case IntentAtom:
		if c.mass[IntentAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need intent atoms", Phase: IntentAtom}

	case AssessmentAtom:
		if c.mass[AssessmentAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need assessment atoms", Phase: AssessmentAtom}

	case PlanAtom:
		if c.mass[PlanAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need plan atoms", Phase: PlanAtom}

	case ExecutionAtom:
		if c.mass[ExecutionAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need execution atoms", Phase: ExecutionAtom}

	case RetrospectionAtom:
		if c.mass[RetrospectionAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
	}
	return Unresolvable, Fortune{Result: Unresolvable, Message: "unknown phase"}
}

func (c *Circuit) advancePhase() {
	prevTriad := TriadOf(c.phase)
	switch c.phase {
	case IntentAtom:
		c.phase = AssessmentAtom
	case AssessmentAtom:
		c.triadSealed[ReasonTriad] = true
		c.phase = PlanAtom
	case PlanAtom:
		c.triadSealed[PlanTriad] = true
		c.phase = ExecutionAtom
	case ExecutionAtom:
		c.phase = RetrospectionAtom
	case RetrospectionAtom:
		c.triadSealed[ActTriad] = true
	}
	_ = prevTriad
}

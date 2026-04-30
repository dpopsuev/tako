package reactivity

import "time"

// Molecule is the substrate the Reactor operates on.
// Reactor = CPU. Molecule = RAM. Focus switch = swap Molecule.
type Molecule struct {
	ID          string
	atoms       map[string]*Atom
	edges       []Edge
	edgeIndex   map[string][]int
	subgraphs   map[AtomType][]string
	taxonomy    map[string][]string
	mass        map[AtomType]int
	sourceMass  map[AtomSource]int
	triadSealed map[Triad]bool
	phase       AtomType
	sealed      bool
	unsealCount int
	createdAt   time.Time
}

// NewMolecule creates a Molecule starting at Intent phase.
func NewMolecule(id string) *Molecule {
	return &Molecule{
		ID:          id,
		atoms:       make(map[string]*Atom),
		edgeIndex:   make(map[string][]int),
		subgraphs:   make(map[AtomType][]string),
		taxonomy:    make(map[string][]string),
		mass:        make(map[AtomType]int),
		sourceMass:  make(map[AtomSource]int),
		triadSealed: make(map[Triad]bool),
		phase:       IntentAtom,
		createdAt:   time.Now(),
	}
}

func (m *Molecule) Phase() AtomType             { return m.phase }
func (m *Molecule) Sealed() bool                { return m.sealed }
func (m *Molecule) Mass(t AtomType) int         { return m.mass[t] }
func (m *Molecule) SourceMass(s AtomSource) int { return m.sourceMass[s] }
func (m *Molecule) CurrentTriad() Triad         { return TriadOf(m.phase) }
func (m *Molecule) TriadSealed(t Triad) bool    { return m.triadSealed[t] }
func (m *Molecule) UnsealCount() int            { return m.unsealCount }

func (m *Molecule) AllTriadsSealed() bool {
	return m.triadSealed[ReasonTriad] && m.triadSealed[PlanTriad] &&
		m.triadSealed[ActTriad] && m.triadSealed[RetrospectTriad]
}

func (m *Molecule) TotalMass() int {
	total := 0
	for _, v := range m.mass {
		total += v
	}
	return total
}

func (m *Molecule) Atoms(t AtomType) []*Atom {
	ids := m.subgraphs[t]
	out := make([]*Atom, 0, len(ids))
	for _, id := range ids {
		if a, ok := m.atoms[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

func (m *Molecule) Atom(id string) (*Atom, bool) {
	a, ok := m.atoms[id]
	return a, ok
}

func (m *Molecule) ByTaxonomy(prefix string) []*Atom {
	var out []*Atom
	for key, ids := range m.taxonomy {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			for _, id := range ids {
				if a, ok := m.atoms[id]; ok {
					out = append(out, a)
				}
			}
		}
	}
	return out
}

func (m *Molecule) AddEdge(from, to string, kind EdgeKind) {
	idx := len(m.edges)
	m.edges = append(m.edges, Edge{From: from, To: to, Kind: kind})
	m.edgeIndex[from] = append(m.edgeIndex[from], idx)
}

func (m *Molecule) EdgesFrom(atomID string) []string {
	indices := m.edgeIndex[atomID]
	out := make([]string, 0, len(indices))
	for _, i := range indices {
		out = append(out, m.edges[i].To)
	}
	return out
}

func (m *Molecule) TypedEdgesFrom(atomID string) []Edge {
	indices := m.edgeIndex[atomID]
	out := make([]Edge, 0, len(indices))
	for _, i := range indices {
		out = append(out, m.edges[i])
	}
	return out
}

func (m *Molecule) Edges() []Edge {
	return append([]Edge(nil), m.edges...)
}

func (m *Molecule) EdgesByKind(kind EdgeKind) []Edge {
	var out []Edge
	for _, e := range m.edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

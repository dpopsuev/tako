package reactivity

import (
	"log/slog"
	"time"
)

type MoleculeEvent struct {
	Kind       string
	MoleculeID string
	Atom       *Atom
	Phase      AtomType
}

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
	emissions        []Emission
	context          any
	createdAt        time.Time
	turns            int
	phaseTransitions int
	catalyst         *Catalyst
	sensorResults    map[string]any
	prevDistance      float64
	deltaDistance     float64
	subscribers      []func(MoleculeEvent)
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

// NewMoleculeWithCatalyst creates a Molecule with structured completion criteria.
func NewMoleculeWithCatalyst(id string, c Catalyst) *Molecule {
	m := NewMolecule(id)
	m.catalyst = &c
	m.sensorResults = make(map[string]any)
	return m
}

func (m *Molecule) Catalyst() *Catalyst { return m.catalyst }

// ReportSensor records a sensor result. If all criteria are met, seals the Molecule.
func (m *Molecule) ReportSensor(key string, value any) {
	if m.sensorResults == nil {
		m.sensorResults = make(map[string]any)
	}
	m.sensorResults[key] = value
	if m.catalyst != nil && m.criteriaMet() {
		m.sealed = true
	}
}

func (m *Molecule) criteriaMet() bool {
	if m.catalyst == nil || len(m.catalyst.Desired) == 0 {
		return false
	}
	for k, expected := range m.catalyst.Desired {
		actual, ok := m.sensorResults[k]
		if !ok || actual != expected {
			return false
		}
	}
	return true
}

func (m *Molecule) Phase() AtomType             { return m.phase }
func (m *Molecule) Sealed() bool                { return m.sealed }
func (m *Molecule) Mass(t AtomType) int         { return m.mass[t] }
func (m *Molecule) SourceMass(s AtomSource) int { return m.sourceMass[s] }
func (m *Molecule) CurrentTriad() Triad         { return m.phase.Triad }
func (m *Molecule) TriadSealed(t Triad) bool    { return m.triadSealed[t] }
func (m *Molecule) UnsealCount() int            { return m.unsealCount }

func (m *Molecule) AllTriadsSealed() bool {
	return m.triadSealed[ThinkTriad] && m.triadSealed[ComposeTriad] &&
		m.triadSealed[ImplementTriad] && m.triadSealed[ReflectTriad]
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

func (m *Molecule) Seal(wish Atom) {
	wish.Type = RetrospectionAtom
	m.atoms[wish.ID] = &wish
	m.subgraphs[RetrospectionAtom] = append(m.subgraphs[RetrospectionAtom], wish.ID)
	m.mass[RetrospectionAtom]++
	if wish.Taxonomy != "" {
		m.taxonomy[wish.Taxonomy] = append(m.taxonomy[wish.Taxonomy], wish.ID)
	}
	m.sealed = true
	m.notify("sealed", nil)
}

func (m *Molecule) Contradict(atom Atom) (bool, *Atom) {
	domain := atomDomain(atom.Taxonomy)
	if domain == "" {
		return false, nil
	}
	for _, existing := range m.atomsByDomain(domain) {
		if existing.ID != atom.ID && existing.Type != atom.Type {
			return true, existing
		}
	}
	return false, nil
}

func (m *Molecule) UnsealTriad(t Triad) {
	switch t {
	case ThinkTriad:
		m.triadSealed[ThinkTriad] = false
		m.triadSealed[ComposeTriad] = false
		m.triadSealed[ImplementTriad] = false
	case ComposeTriad:
		m.triadSealed[ComposeTriad] = false
		m.triadSealed[ImplementTriad] = false
	case ImplementTriad:
		m.triadSealed[ImplementTriad] = false
	}
	m.unsealCount++
}

func (m *Molecule) InsertAtom(atom Atom) {
	m.atoms[atom.ID] = &atom
	m.subgraphs[atom.Type] = append(m.subgraphs[atom.Type], atom.ID)
	m.mass[atom.Type]++
	m.sourceMass[atom.Source]++
	if atom.Taxonomy != "" {
		m.taxonomy[atom.Taxonomy] = append(m.taxonomy[atom.Taxonomy], atom.ID)
	}
	for _, target := range atom.Targets {
		m.AddEdge(atom.ID, target, Reference)
	}
	m.notify("atom_inserted", &atom)
}

func (m *Molecule) SetPhase(p AtomType) {
	if p != m.phase {
		m.phaseTransitions++
		m.phase = p
		m.notify("phase_changed", nil)
		return
	}
	m.phase = p
}
func (m *Molecule) SealTriad(t Triad) { m.triadSealed[t] = true }
func (m *Molecule) IsSealed() bool    { return m.sealed }

func (m *Molecule) Tick() {
	d := m.Distance()
	m.deltaDistance = d - m.prevDistance
	m.prevDistance = d
	m.turns++
}

// DeltaDistance returns the change in distance since last turn.
// Negative = getting closer (good). Positive = getting further (bad). Zero = stuck.
func (m *Molecule) DeltaDistance() float64 { return m.deltaDistance }
func (m *Molecule) Turns() int { return m.turns }

// Momentum returns the ratio of phase transitions to turns spent.
// 0 = no progress (subcritical). 1 = one transition per turn (critical).
// >1 = multiple transitions per turn (supercritical).
func (m *Molecule) Momentum() float64 {
	if m.turns == 0 {
		return 0
	}
	return float64(m.phaseTransitions) / float64(m.turns)
}

// Distance returns progress toward the Desired state.
// With Catalyst: vector distance (0.0 = at Desired, 1.0 = at Current).
// Without Catalyst: fraction of unfilled phases (fallback).
func (m *Molecule) Distance() float64 {
	if m.catalyst != nil && len(m.catalyst.Desired) > 0 {
		total := len(m.catalyst.Desired)
		met := 0
		for k, expected := range m.catalyst.Desired {
			if actual, ok := m.sensorResults[k]; ok && actual == expected {
				met++
			}
		}
		return float64(total-met) / float64(total)
	}
	all := AllAtomTypes()
	if len(all) == 0 {
		return 0
	}
	unfilled := 0
	for _, at := range all {
		if m.mass[at] == 0 {
			unfilled++
		}
	}
	return float64(unfilled) / float64(len(all))
}

// Residual returns the per-dimension gap between actual state and Desired.
// 0 = dimension met, 1 = dimension unmet. Returns nil without Catalyst.
func (m *Molecule) Residual() map[string]float64 {
	if m.catalyst == nil || len(m.catalyst.Desired) == 0 {
		return nil
	}
	r := make(map[string]float64, len(m.catalyst.Desired))
	for k, expected := range m.catalyst.Desired {
		if actual, ok := m.sensorResults[k]; ok && actual == expected {
			r[k] = 0
		} else {
			r[k] = 1
		}
	}
	return r
}

func (m *Molecule) Subscribe(fn func(MoleculeEvent)) {
	m.subscribers = append(m.subscribers, fn)
}

func (m *Molecule) notify(kind string, atom *Atom) {
	if len(m.subscribers) == 0 {
		return
	}
	event := MoleculeEvent{
		Kind:       kind,
		MoleculeID: m.ID,
		Atom:       atom,
		Phase:      m.phase,
	}
	for _, fn := range m.subscribers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("molecule.subscriber_panic",
						slog.String("molecule", m.ID),
						slog.String("event", kind),
						slog.Any("panic", r))
				}
			}()
			fn(event)
		}()
	}
}

func (m *Molecule) Context() any            { return m.context }
func (m *Molecule) SetContext(v any)        { m.context = v }

func (m *Molecule) Emit(e Emission)        { m.emissions = append(m.emissions, e) }

func (m *Molecule) Emissions() []Emission {
	return append([]Emission(nil), m.emissions...)
}

func (m *Molecule) DrainEmissions() []Emission {
	out := m.emissions
	m.emissions = nil
	return out
}

func (m *Molecule) atomsByDomain(domain string) []*Atom {
	var out []*Atom
	for _, a := range m.atoms {
		if atomDomain(a.Taxonomy) == domain {
			out = append(out, a)
		}
	}
	return out
}

func atomDomain(taxonomy string) string {
	for i := len(taxonomy) - 1; i >= 0; i-- {
		if taxonomy[i] == '.' {
			return taxonomy[i+1:]
		}
	}
	return ""
}

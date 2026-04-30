package reactivity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

// Reactor is the molecular machine. Stateless. Processes Molecules.
type Reactor struct {
	pool   ergograph.Pool
	tracer trace.Tracer
}

// NewReactor creates a Reactor (the machine).
func NewReactor(opts ...ReactorOption) *Reactor {
	c := &Reactor{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ReactorOption configures a Reactor.
type ReactorOption func(*Reactor)

func WithPool(pool ergograph.Pool) ReactorOption {
	return func(c *Reactor) { c.pool = pool }
}

func WithTracer(tracer trace.Tracer) ReactorOption {
	return func(c *Reactor) { c.tracer = tracer }
}

// Add inserts an atom into the molecule, creates edges, updates indexes, runs Assert.
func (c *Reactor) Add(m *Molecule, atom Atom) (AssertResult, Fortune) {
	if m.sealed {
		return Unresolvable, Fortune{Result: Unresolvable, Message: "molecule is sealed"}
	}

	if atom.Type > m.phase && atom.Type != AssessmentAtom {
		return Incompatible, Fortune{
			Result:  Incompatible,
			Message: fmt.Sprintf("molecule is in %s phase, cannot accept future %s atom", m.phase, atom.Type),
			Phase:   m.phase,
		}
	}

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

	result, fortune := c.assertPhase(m)
	if result == Pass {
		c.advancePhase(m)
	}
	c.record("circuit.add", map[string]string{
		labelAtom:     atom.ID,
		labelType:     atom.Type.String(),
		labelTaxonomy: atom.Taxonomy,
		labelResult:   result.String(),
	})
	c.span("circuit.add")
	return result, fortune
}

// Seal marks the molecule as complete with a Wish atom.
func (c *Reactor) Seal(m *Molecule, wish Atom) {
	wish.Type = RetrospectionAtom
	m.atoms[wish.ID] = &wish
	m.subgraphs[RetrospectionAtom] = append(m.subgraphs[RetrospectionAtom], wish.ID)
	m.mass[RetrospectionAtom]++
	if wish.Taxonomy != "" {
		m.taxonomy[wish.Taxonomy] = append(m.taxonomy[wish.Taxonomy], wish.ID)
	}
	m.sealed = true
	c.record("circuit.seal", map[string]string{
		labelWish:  wish.ID,
		labelDepth: m.CurrentTriad().String(),
		labelMass:  fmt.Sprintf("%d", m.TotalMass()),
	})
	c.span("circuit.seal")
}

// Contradict checks if an atom contradicts an existing atom about the same concern.
func (c *Reactor) Contradict(m *Molecule, atom Atom) (bool, *Atom) {
	domain := taxonomyDomain(atom.Taxonomy)
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

// UnsealTriad unseals a triad and all lower triads (cascade down).
func (c *Reactor) UnsealTriad(m *Molecule, t Triad) {
	switch t {
	case ReasonTriad:
		m.triadSealed[ReasonTriad] = false
		m.triadSealed[PlanTriad] = false
		m.triadSealed[ActTriad] = false
	case PlanTriad:
		m.triadSealed[PlanTriad] = false
		m.triadSealed[ActTriad] = false
	case ActTriad:
		m.triadSealed[ActTriad] = false
	}
	m.unsealCount++
	c.record("circuit.triad.unseal", map[string]string{labelTriad: t.String()})
	c.span("circuit.triad.unseal")
}

func (m *Molecule) atomsByDomain(domain string) []*Atom {
	var out []*Atom
	for _, a := range m.atoms {
		if taxonomyDomain(a.Taxonomy) == domain {
			out = append(out, a)
		}
	}
	return out
}

func taxonomyDomain(taxonomy string) string {
	parts := strings.Split(taxonomy, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func (c *Reactor) assertPhase(m *Molecule) (AssertResult, Fortune) {
	switch m.phase {
	case IntentAtom:
		if m.mass[IntentAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need intent atoms", Phase: IntentAtom}
	case AssessmentAtom:
		if m.mass[AssessmentAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need assessment atoms", Phase: AssessmentAtom}
	case PlanAtom:
		if m.mass[PlanAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need plan atoms", Phase: PlanAtom}
	case ExecutionAtom:
		if m.mass[ExecutionAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need execution atoms", Phase: ExecutionAtom}
	case RetrospectionAtom:
		if m.mass[RetrospectionAtom] > 0 {
			return Pass, Fortune{}
		}
		return Insufficient, Fortune{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
	}
	return Unresolvable, Fortune{Result: Unresolvable, Message: "unknown phase"}
}

func (c *Reactor) advancePhase(m *Molecule) {
	switch m.phase {
	case IntentAtom:
		m.phase = AssessmentAtom
	case AssessmentAtom:
		m.triadSealed[ReasonTriad] = true
		c.record("circuit.triad.seal", map[string]string{labelTriad: ReasonTriad.String()})
		c.span("circuit.triad.seal")
		m.phase = PlanAtom
	case PlanAtom:
		m.triadSealed[PlanTriad] = true
		c.record("circuit.triad.seal", map[string]string{labelTriad: PlanTriad.String()})
		c.span("circuit.triad.seal")
		m.phase = ExecutionAtom
	case ExecutionAtom:
		m.triadSealed[ActTriad] = true
		c.record("circuit.triad.seal", map[string]string{labelTriad: ActTriad.String()})
		c.span("circuit.triad.seal")
		m.phase = RetrospectionAtom
	case RetrospectionAtom:
		m.triadSealed[RetrospectTriad] = true
		c.record("circuit.triad.seal", map[string]string{labelTriad: RetrospectTriad.String()})
		c.span("circuit.triad.seal")
	}
}

const (
	labelAtom     = "atom"
	labelType     = "type"
	labelTaxonomy = "taxonomy"
	labelResult   = "result"
	labelTriad    = "triad"
	labelWish     = "wish"
	labelDepth    = "depth"
	labelMass     = "total_mass"
	labelError    = "error"
)

func (c *Reactor) record(action string, labels map[string]string) {
	if c.pool == nil {
		return
	}
	if err := c.pool.Append(ergograph.Record{
		Action:    action,
		Timestamp: time.Now(),
		Labels:    labels,
	}); err != nil {
		slog.WarnContext(context.Background(), "reactivity: record failed", slog.String(labelAtom, action), slog.Any(labelError, err))
	}
}

func (c *Reactor) span(name string) {
	if c.tracer == nil {
		return
	}
	_, s := c.tracer.Start(context.Background(), name)
	s.End()
}

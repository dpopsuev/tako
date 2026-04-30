package reactivity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

// Reactor is the processing interface. Same at every level — leaf or composite.
type Reactor interface {
	React(m *Molecule, atom Atom) (AssertResult, Fortune)
}

// NoOp passes the atom through without processing. Ablation baseline.
type NoOp struct{}

func (NoOp) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	m.InsertAtom(atom)
	return Pass, Fortune{}
}

type reasonReactor struct{}

func (reasonReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	if m.mass[AssessmentAtom] > 0 {
		m.SealTriad(ReasonTriad)
		m.SetPhase(PlanAtom)
		return Pass, Fortune{}
	}
	if m.phase == IntentAtom && m.mass[IntentAtom] > 0 {
		m.SetPhase(AssessmentAtom)
		return Pass, Fortune{}
	}
	return Insufficient, Fortune{Result: Insufficient, Message: "need reason atoms", Phase: m.phase}
}

type planReactor struct{}

func (planReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	if m.mass[PlanAtom] > 0 {
		m.SealTriad(PlanTriad)
		m.SetPhase(ExecutionAtom)
		return Pass, Fortune{}
	}
	return Insufficient, Fortune{Result: Insufficient, Message: "need plan atoms", Phase: PlanAtom}
}

type actReactor struct{}

func (actReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	if m.mass[ExecutionAtom] > 0 {
		m.SealTriad(ActTriad)
		m.SetPhase(RetrospectionAtom)
		return Pass, Fortune{}
	}
	return Insufficient, Fortune{Result: Insufficient, Message: "need execution atoms", Phase: ExecutionAtom}
}

type retrospectReactor struct{}

func (retrospectReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	if m.mass[RetrospectionAtom] > 0 {
		m.SealTriad(RetrospectTriad)
		return Pass, Fortune{}
	}
	return Insufficient, Fortune{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
}

// CompositeReactor composes 4 nested Reactors — one per triad.
// Same Reactor interface. Adds observability + lifecycle helpers.
type CompositeReactor struct {
	triads map[Triad]Reactor
	pool   ergograph.Pool
	tracer trace.Tracer
}

type ReactorOption func(*CompositeReactor)

func WithPool(pool ergograph.Pool) ReactorOption {
	return func(c *CompositeReactor) { c.pool = pool }
}

func WithTracer(tracer trace.Tracer) ReactorOption {
	return func(c *CompositeReactor) { c.tracer = tracer }
}

func WithTriad(t Triad, r Reactor) ReactorOption {
	return func(c *CompositeReactor) { c.triads[t] = r }
}

func NewReactor(opts ...ReactorOption) *CompositeReactor {
	c := &CompositeReactor{
		triads: map[Triad]Reactor{
			ReasonTriad:    reasonReactor{},
			PlanTriad:      planReactor{},
			ActTriad:       actReactor{},
			RetrospectTriad: retrospectReactor{},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// React inserts an atom and delegates to the appropriate triad reactor.
func (c *CompositeReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
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

	m.InsertAtom(atom)

	triad := TriadOf(atom.Type)
	r := c.triads[triad]
	result, fortune := r.React(m, atom)

	c.record("reactor.react", map[string]string{
		labelAtom:     atom.ID,
		labelType:     atom.Type.String(),
		labelTaxonomy: atom.Taxonomy,
		labelResult:   result.String(),
		labelTriad:    triad.String(),
	})
	c.span("reactor.react")
	return result, fortune
}

// Add is an alias for React — backward compatibility.
func (c *CompositeReactor) Add(m *Molecule, atom Atom) (AssertResult, Fortune) {
	return c.React(m, atom)
}

// Seal marks the molecule as complete with a Wish atom.
func (c *CompositeReactor) Seal(m *Molecule, wish Atom) {
	m.Seal(wish)
	c.record("reactor.seal", map[string]string{
		labelWish: wish.ID,
		labelDepth: m.CurrentTriad().String(),
		labelMass:  fmt.Sprintf("%d", m.TotalMass()),
	})
	c.span("reactor.seal")
}

// Contradict checks if an atom contradicts existing atoms.
func (c *CompositeReactor) Contradict(m *Molecule, atom Atom) (bool, *Atom) {
	return m.Contradict(atom)
}

// UnsealTriad unseals a triad with cascade.
func (c *CompositeReactor) UnsealTriad(m *Molecule, t Triad) {
	m.UnsealTriad(t)
	c.record("reactor.triad.unseal", map[string]string{labelTriad: t.String()})
	c.span("reactor.triad.unseal")
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

func (c *CompositeReactor) record(action string, labels map[string]string) {
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

func (c *CompositeReactor) span(name string) {
	if c.tracer == nil {
		return
	}
	_, s := c.tracer.Start(context.Background(), name)
	s.End()
}

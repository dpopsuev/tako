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
	React(m *Molecule, atom Atom) (YieldKind, Yield)
}

// Damper passes the atom through without processing. Ablation baseline.
type Damper struct{}

func (Damper) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	m.InsertAtom(atom)
	return Pass, Yield{}
}

type reasonReactor struct{}

func (reasonReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.mass[AssessmentAtom] > 0 {
		m.SealTriad(ReasonTriad)
		m.SetPhase(PlanAtom)
		return Pass, Yield{}
	}
	if m.phase == IntentAtom && m.mass[IntentAtom] > 0 {
		m.SetPhase(AssessmentAtom)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need reason atoms", Phase: m.phase}
}

type planReactor struct{}

func (planReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.mass[PlanAtom] > 0 {
		m.SealTriad(PlanTriad)
		m.SetPhase(ExecutionAtom)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need plan atoms", Phase: PlanAtom}
}

type actReactor struct{}

func (actReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.mass[ExecutionAtom] > 0 {
		m.SealTriad(ActTriad)
		m.SetPhase(RetrospectionAtom)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need execution atoms", Phase: ExecutionAtom}
}

type retrospectReactor struct{}

func (retrospectReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.mass[RetrospectionAtom] > 0 {
		m.SealTriad(RetrospectTriad)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
}

// Core composes 4 nested Reactors — one per triad.
// Same Reactor interface. Adds observability + lifecycle helpers.
type Core struct {
	triads map[Triad]Reactor
	pool   ergograph.Pool
	tracer trace.Tracer
}

type ReactorOption func(*Core)

func WithPool(pool ergograph.Pool) ReactorOption {
	return func(c *Core) { c.pool = pool }
}

func WithTracer(tracer trace.Tracer) ReactorOption {
	return func(c *Core) { c.tracer = tracer }
}

func WithTriad(t Triad, r Reactor) ReactorOption {
	return func(c *Core) { c.triads[t] = r }
}

func NewReactor(opts ...ReactorOption) *Core {
	c := &Core{
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
func (c *Core) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.sealed {
		return Unresolvable, Yield{Result: Unresolvable, Message: "molecule is sealed"}
	}

	if atom.Type > m.phase && atom.Type != AssessmentAtom {
		return Incompatible, Yield{
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
func (c *Core) Add(m *Molecule, atom Atom) (YieldKind, Yield) {
	return c.React(m, atom)
}

// Seal marks the molecule as complete with a Wish atom.
func (c *Core) Seal(m *Molecule, wish Atom) {
	m.Seal(wish)
	c.record("reactor.seal", map[string]string{
		labelWish: wish.ID,
		labelDepth: m.CurrentTriad().String(),
		labelMass:  fmt.Sprintf("%d", m.TotalMass()),
	})
	c.span("reactor.seal")
}

// Contradict checks if an atom contradicts existing atoms.
func (c *Core) Contradict(m *Molecule, atom Atom) (bool, *Atom) {
	return m.Contradict(atom)
}

// UnsealTriad unseals a triad with cascade.
func (c *Core) UnsealTriad(m *Molecule, t Triad) {
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

func (c *Core) record(action string, labels map[string]string) {
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

func (c *Core) span(name string) {
	if c.tracer == nil {
		return
	}
	_, s := c.tracer.Start(context.Background(), name)
	s.End()
}

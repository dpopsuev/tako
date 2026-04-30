package reactivity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

// Reactor is the processing interface. Same at every level — leaf, triad, or core.
type Reactor interface {
	React(m *Molecule, atom Atom) (YieldKind, Yield)
}

// Damper passes the atom through without processing. Ablation baseline (control rod).
type Damper struct{}

func (Damper) React(m *Molecule, _ Atom) (YieldKind, Yield) {
	return Pass, Yield{}
}

// TriadReactor composes 3 node Reactors — thesis, antithesis, synthesis.
// Same Reactor interface. Each floor of the Core is a TriadReactor.
type TriadReactor struct {
	triad      Triad
	thesis     Reactor
	antithesis Reactor
	synthesis  Reactor
	nodes      [3]AtomType
	nextPhase  AtomType
}

func NewTriadReactor(triad Triad, nodes [3]AtomType, nextPhase AtomType) *TriadReactor {
	return &TriadReactor{
		triad:      triad,
		thesis:     Damper{},
		antithesis: Damper{},
		synthesis:  Damper{},
		nodes:      nodes,
		nextPhase:  nextPhase,
	}
}

func (t *TriadReactor) WithThesis(r Reactor) *TriadReactor     { t.thesis = r; return t }
func (t *TriadReactor) WithAntithesis(r Reactor) *TriadReactor { t.antithesis = r; return t }
func (t *TriadReactor) WithSynthesis(r Reactor) *TriadReactor  { t.synthesis = r; return t }

func (t *TriadReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	pos := PositionOf(atom.Type)
	switch pos {
	case ThesisPosition:
		t.thesis.React(m, atom)
	case AntithesisPosition:
		t.antithesis.React(m, atom)
	case SynthesisPosition:
		t.synthesis.React(m, atom)
	}

	if m.mass[t.nodes[SynthesisPosition]] > 0 {
		m.SealTriad(t.triad)
		m.SetPhase(t.nextPhase)
		return Pass, Yield{}
	}
	if m.mass[t.nodes[AntithesisPosition]] > 0 && m.phase == t.nodes[ThesisPosition] {
		m.SetPhase(t.nodes[AntithesisPosition])
		return Pass, Yield{}
	}
	if m.mass[t.nodes[AntithesisPosition]] > 0 && m.phase == t.nodes[AntithesisPosition] {
		m.SetPhase(t.nodes[SynthesisPosition])
		return Pass, Yield{}
	}
	if m.mass[t.nodes[ThesisPosition]] > 0 && m.phase == t.nodes[ThesisPosition] {
		m.SetPhase(t.nodes[AntithesisPosition])
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: fmt.Sprintf("need %s atoms", t.triad), Phase: m.phase}
}

// Reflection is the Retrospect sink's node. Seals the molecule.
type Reflection struct{}

func (Reflection) React(m *Molecule, _ Atom) (YieldKind, Yield) {
	if m.mass[RetrospectionAtom] > 0 {
		m.SealTriad(RetrospectTriad)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
}

// Core composes 3 floor TriadReactors + Retrospect sink.
// Router (ingress) → Reason → Formation → Action → Reflection (egress).
type Core struct {
	floors map[Triad]Reactor
	sink   Reactor
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
	return func(c *Core) { c.floors[t] = r }
}

func WithSink(r Reactor) ReactorOption {
	return func(c *Core) { c.sink = r }
}

func NewReactor(opts ...ReactorOption) *Core {
	c := &Core{
		floors: map[Triad]Reactor{
			ReasonTriad: NewTriadReactor(ReasonTriad,
				[3]AtomType{IntentAtom, AssessmentAtom, UnderstandingAtom},
				PlanAtom,
			),
			PlanTriad: NewTriadReactor(PlanTriad,
				[3]AtomType{PlanAtom, RiskAtom, StrategyAtom},
				ExecutionAtom,
			),
			ActTriad: NewTriadReactor(ActTriad,
				[3]AtomType{ExecutionAtom, ObservationAtom, AdaptationAtom},
				RetrospectionAtom,
			),
		},
		sink: Reflection{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// React is the Router — ingress node of Core. Routes atom to the right floor or sink.
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
	var r Reactor
	if triad == RetrospectTriad {
		r = c.sink
	} else {
		r = c.floors[triad]
	}
	result, yield := r.React(m, atom)

	c.record("reactor.react", map[string]string{
		labelAtom:     atom.ID,
		labelType:     atom.Type.String(),
		labelTaxonomy: atom.Taxonomy,
		labelResult:   result.String(),
		labelTriad:    triad.String(),
	})
	c.span("reactor.react")
	return result, yield
}

// Add is an alias for React — backward compatibility.
func (c *Core) Add(m *Molecule, atom Atom) (YieldKind, Yield) {
	return c.React(m, atom)
}

// Seal marks the molecule as complete with a Wish atom.
func (c *Core) Seal(m *Molecule, wish Atom) {
	m.Seal(wish)
	c.record("reactor.seal", map[string]string{
		labelWish:  wish.ID,
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

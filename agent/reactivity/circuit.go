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

// Directive is a tuning prompt attached to a Reactor node.
type Directive string

// TriadReactor composes 3 Nodes — thesis, antithesis, synthesis.
// Same Reactor interface. Each floor of the Core is a TriadReactor.
type TriadReactor struct {
	triad     Triad
	thesis    *Node
	antithesis *Node
	synthesis  *Node
	nextPhase AtomType
}

func NewTriadReactor(triad Triad, nextPhase AtomType) *TriadReactor {
	return &TriadReactor{
		triad:      triad,
		thesis:     GimpedNode(AtomType{triad, ThesisPosition}),
		antithesis: GimpedNode(AtomType{triad, AntithesisPosition}),
		synthesis:  GimpedNode(AtomType{triad, SynthesisPosition}),
		nextPhase:  nextPhase,
	}
}

func (t *TriadReactor) Thesis() *Node     { return t.thesis }
func (t *TriadReactor) Antithesis() *Node  { return t.antithesis }
func (t *TriadReactor) Synthesis() *Node   { return t.synthesis }

func (t *TriadReactor) Node(pos DialecticPosition) *Node {
	switch pos {
	case ThesisPosition:
		return t.thesis
	case AntithesisPosition:
		return t.antithesis
	case SynthesisPosition:
		return t.synthesis
	default:
		return t.thesis
	}
}

func (t *TriadReactor) phase(pos DialecticPosition) AtomType {
	return AtomType{t.triad, pos}
}

func (t *TriadReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	node := t.Node(atom.Type.Position)
	node.React(m, atom)

	if m.mass[t.phase(SynthesisPosition)] > 0 {
		m.SealTriad(t.triad)
		m.SetPhase(t.nextPhase)
		return Pass, Yield{}
	}
	if m.mass[t.phase(AntithesisPosition)] > 0 && m.phase == t.phase(ThesisPosition) {
		m.SetPhase(t.phase(AntithesisPosition))
		return Pass, Yield{}
	}
	if m.mass[t.phase(AntithesisPosition)] > 0 && m.phase == t.phase(AntithesisPosition) {
		m.SetPhase(t.phase(SynthesisPosition))
		return Pass, Yield{}
	}
	if m.mass[t.phase(ThesisPosition)] > 0 && m.phase == t.phase(ThesisPosition) {
		m.SetPhase(t.phase(AntithesisPosition))
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: fmt.Sprintf("need %s atoms", t.triad), Phase: m.phase}
}

// Reflection is the Retrospect sink's node. Seals the molecule.
type Reflection struct{}

func (Reflection) React(m *Molecule, _ Atom) (YieldKind, Yield) {
	if m.mass[RetrospectionAtom] > 0 {
		m.SealTriad(ReflectTriad)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need retrospection atoms", Phase: RetrospectionAtom}
}

// Core composes 3 floor TriadReactors + Reflect egress.
// Cognize (ingress) → Think → Compose → Implement → Reflect (egress).
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

func WithDirective(phase AtomType, directive Directive) ReactorOption {
	return func(c *Core) {
		if n := c.node(phase); n != nil {
			n.AddDirective(directive)
		}
	}
}

func NewReactor(opts ...ReactorOption) *Core {
	c := &Core{
		floors: map[Triad]Reactor{
			ThinkTriad:     NewTriadReactor(ThinkTriad, AtomType{ComposeTriad, ThesisPosition}),
			ComposeTriad:   NewTriadReactor(ComposeTriad, AtomType{ImplementTriad, ThesisPosition}),
			ImplementTriad: NewTriadReactor(ImplementTriad, RetrospectionAtom),
		},
		sink: Reflection{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Core) AddDirective(phase AtomType, directive Directive) {
	if n := c.node(phase); n != nil {
		n.AddDirective(directive)
	}
}

func (c *Core) Directives(phase AtomType) []Directive {
	if n := c.node(phase); n != nil {
		return n.Directives()
	}
	return nil
}

func (c *Core) Node(phase AtomType) *Node {
	return c.node(phase)
}

func (c *Core) node(phase AtomType) *Node {
	triad := phase.Triad
	floor, ok := c.floors[triad]
	if !ok {
		return nil
	}
	tr, ok := floor.(*TriadReactor)
	if !ok {
		return nil
	}
	return tr.Node(phase.Position)
}

// React is the Cognizer — ingress node of Core. Routes atom to the right floor or sink.
func (c *Core) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	if m.sealed {
		return Unresolvable, Yield{Result: Unresolvable, Message: "molecule is sealed"}
	}

	if atom.Type.Sequence() > m.phase.Sequence() && atom.Type != AssessmentAtom {
		return Incompatible, Yield{
			Result:  Incompatible,
			Message: fmt.Sprintf("molecule is in %s phase, cannot accept future %s atom", m.phase, atom.Type),
			Phase:   m.phase,
		}
	}

	m.InsertAtom(atom)

	triad := atom.Type.Triad
	var r Reactor
	if triad == ReflectTriad {
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

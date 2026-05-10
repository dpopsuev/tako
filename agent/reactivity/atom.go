package reactivity

import "time"

// AtomType identifies an atom's position in the Reactor.
// Composed of Triad (which floor) and DialecticPosition (which role).
type AtomType struct {
	Triad    Triad
	Position DialecticPosition
}

func (a AtomType) Sequence() int {
	return int(a.Triad)*3 + int(a.Position)
}

func (a AtomType) String() string {
	for _, named := range namedAtoms {
		if named.atom == a {
			return named.label
		}
	}
	return a.Triad.String() + "." + a.Position.String()
}

type namedAtom struct {
	atom  AtomType
	label string
}

var namedAtoms = []namedAtom{
	{IntentAtom, "intent"},
	{AssessmentAtom, "assessment"},
	{KnowledgeAtom, "knowledge"},
	{ExpansionAtom, "expansion"},
	{ReductionAtom, "reduction"},
	{SelectionAtom, "selection"},
	{ExecutionAtom, "execution"},
	{AcclimationAtom, "acclimation"},
	{RefinementAtom, "refinement"},
	{RetrospectionAtom, "retrospection"},
}

var (
	// Think triad
	IntentAtom     = AtomType{ThinkTriad, ThesisPosition}
	AssessmentAtom = AtomType{ThinkTriad, AntithesisPosition}
	KnowledgeAtom  = AtomType{ThinkTriad, SynthesisPosition}

	// Compose triad
	ExpansionAtom = AtomType{ComposeTriad, ThesisPosition}
	ReductionAtom = AtomType{ComposeTriad, AntithesisPosition}
	SelectionAtom = AtomType{ComposeTriad, SynthesisPosition}

	// Implement triad
	ExecutionAtom   = AtomType{ImplementTriad, ThesisPosition}
	AcclimationAtom = AtomType{ImplementTriad, AntithesisPosition}
	RefinementAtom  = AtomType{ImplementTriad, SynthesisPosition}

	// Reflect egress
	RetrospectionAtom = AtomType{ReflectTriad, SynthesisPosition}
)

func AllAtomTypes() []AtomType {
	out := make([]AtomType, len(namedAtoms))
	for i, n := range namedAtoms {
		out[i] = n.atom
	}
	return out
}

// AtomSource indicates where this atom came from.
type AtomSource int

const (
	Fresh       AtomSource = iota
	Recollected
	Received
	Instrument
	Human
)

// Atom is a single knowledge node in the Molecule graph.
type Atom struct {
	ID         string
	Type       AtomType
	Source     AtomSource
	Taxonomy   string
	Content    []byte
	Targets    []string
	Dimensions []string
	Embedding  []float64
	CreatedAt  time.Time
}

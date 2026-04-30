package reactivity

// Triad groups nodes into Think, Compose, Implement.
type Triad int

const (
	ThinkTriad   Triad = iota
	ComposeTriad
	ImplementTriad
	ReflectTriad
)

func (t Triad) String() string {
	switch t {
	case ThinkTriad:
		return "think"
	case ComposeTriad:
		return "compose"
	case ImplementTriad:
		return "implement"
	case ReflectTriad:
		return "reflect"
	default:
		return "unknown"
	}
}

// DialecticPosition is the role of a node within a triad.
type DialecticPosition int

const (
	ThesisPosition     DialecticPosition = 0
	AntithesisPosition DialecticPosition = 1
	SynthesisPosition  DialecticPosition = 2
)

func (p DialecticPosition) String() string {
	switch p {
	case ThesisPosition:
		return "thesis"
	case AntithesisPosition:
		return "antithesis"
	case SynthesisPosition:
		return "synthesis"
	default:
		return "unknown"
	}
}

// TriadNodes returns the AtomTypes that belong to a triad.
func TriadNodes(t Triad) []AtomType {
	return []AtomType{
		{t, ThesisPosition},
		{t, AntithesisPosition},
		{t, SynthesisPosition},
	}
}

// UnsealKind is a named resynthesis request targeting a synthesis node on the spine.
type UnsealKind int

const (
	Recognize  UnsealKind = iota
	Rethink
	Recompose
	Reaction
)

func (u UnsealKind) String() string {
	switch u {
	case Recognize:
		return "recognize"
	case Rethink:
		return "rethink"
	case Recompose:
		return "recompose"
	case Reaction:
		return "reaction"
	default:
		return "unknown"
	}
}

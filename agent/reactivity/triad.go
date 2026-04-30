package reactivity

// Triad groups nodes into Think, Compose, Action.
type Triad int

const (
	ThinkTriad   Triad = iota
	ComposeTriad
	ActionTriad
	ReflectTriad
)

func (t Triad) String() string {
	switch t {
	case ThinkTriad:
		return "think"
	case ComposeTriad:
		return "compose"
	case ActionTriad:
		return "action"
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

// TriadOf returns which triad an AtomType belongs to.
func TriadOf(t AtomType) Triad {
	switch t {
	case KnowledgeAtom, IntentAtom, AssessmentAtom:
		return ThinkTriad
	case SelectionAtom, ExpansionAtom, ReductionAtom:
		return ComposeTriad
	case RefinementAtom, ExecutionAtom, AcclimationAtom:
		return ActionTriad
	case RetrospectionAtom:
		return ReflectTriad
	default:
		return ThinkTriad
	}
}

// TriadNodes returns the AtomTypes that belong to a triad.
func TriadNodes(t Triad) []AtomType {
	switch t {
	case ThinkTriad:
		return []AtomType{KnowledgeAtom, IntentAtom, AssessmentAtom}
	case ComposeTriad:
		return []AtomType{SelectionAtom, ExpansionAtom, ReductionAtom}
	case ActionTriad:
		return []AtomType{RefinementAtom, ExecutionAtom, AcclimationAtom}
	case ReflectTriad:
		return []AtomType{RetrospectionAtom}
	default:
		return nil
	}
}

// PositionOf returns the dialectic position of an AtomType within its triad.
func PositionOf(t AtomType) DialecticPosition {
	switch t {
	case KnowledgeAtom, SelectionAtom, RefinementAtom:
		return SynthesisPosition
	case IntentAtom, ExpansionAtom, ExecutionAtom:
		return ThesisPosition
	case AssessmentAtom, ReductionAtom, AcclimationAtom:
		return AntithesisPosition
	default:
		return SynthesisPosition
	}
}

// UnsealKind is a named resynthesis request targeting a synthesis node on the spine.
type UnsealKind int

const (
	Recognize  UnsealKind = iota // Cognize was wrong, re-cognize to different Molecule
	Rethink                      // Selection was wrong, resynthesize from Knowledge
	Recompose                    // Refinement was wrong, resynthesize from Selection
	Reaction                     // Reflection incomplete, resynthesize from Refinement
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

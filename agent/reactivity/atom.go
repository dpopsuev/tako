package reactivity

import "time"

// AtomType is the phase that produced this Atom.
type AtomType int

const (
	// Think triad
	IntentAtom     AtomType = iota // thesis
	AssessmentAtom                  // antithesis
	KnowledgeAtom                   // synthesis (Recall else Consolidate)

	// Compose triad
	ExpansionAtom // thesis
	ReductionAtom // antithesis
	SelectionAtom // synthesis

	// Action triad
	ExecutionAtom   // thesis
	AcclimationAtom // antithesis
	RefinementAtom  // synthesis

	// Reflect egress
	RetrospectionAtom
)

func (t AtomType) String() string {
	switch t {
	case KnowledgeAtom:
		return "knowledge"
	case IntentAtom:
		return "intent"
	case AssessmentAtom:
		return "assessment"
	case SelectionAtom:
		return "selection"
	case ExpansionAtom:
		return "expansion"
	case ReductionAtom:
		return "reduction"
	case RefinementAtom:
		return "refinement"
	case ExecutionAtom:
		return "execution"
	case AcclimationAtom:
		return "acclimation"
	case RetrospectionAtom:
		return "retrospection"
	default:
		return "unknown"
	}
}

// AtomSource indicates where this atom came from.
type AtomSource int

const (
	Fresh       AtomSource = iota // produced by LLM or instrument this cycle
	Recollected                   // pulled from Memory Mesh (prior knowledge)
	Received                      // from another agent via Discourse
	Instrument                    // from instrument execution result
	Human                         // from HITL response
)

// Atom is a single knowledge node in the Molecule graph.
type Atom struct {
	ID        string
	Type      AtomType
	Source    AtomSource
	Taxonomy  string
	Content   []byte
	Targets   []string
	CreatedAt time.Time
}

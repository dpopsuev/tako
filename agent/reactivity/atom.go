package reactivity

import "time"

// AtomType is the phase that produced this Atom.
type AtomType int

const (
	// Reason triad
	IntentAtom        AtomType = iota // thesis
	AssessmentAtom                     // antithesis
	UnderstandingAtom                  // synthesis

	// Formation triad
	PlanAtom     // thesis
	RiskAtom     // antithesis
	StrategyAtom // synthesis

	// Action triad
	ExecutionAtom   // thesis
	ObservationAtom // antithesis
	AdaptationAtom  // synthesis

	// Retrospect sink
	RetrospectionAtom
)

func (t AtomType) String() string {
	switch t {
	case IntentAtom:
		return "intent"
	case AssessmentAtom:
		return "assessment"
	case UnderstandingAtom:
		return "understanding"
	case PlanAtom:
		return "plan" //nolint:goconst // same word, different semantic (AtomType vs Triad)
	case RiskAtom:
		return "risk"
	case StrategyAtom:
		return "strategy"
	case ExecutionAtom:
		return "execution"
	case ObservationAtom:
		return "observation"
	case AdaptationAtom:
		return "adaptation"
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

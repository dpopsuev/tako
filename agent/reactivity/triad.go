package reactivity

// Triad groups nodes into Reason, Plan, Act.
type Triad int

const (
	ReasonTriad    Triad = iota
	PlanTriad
	ActTriad
	RetrospectTriad
)

func (t Triad) String() string {
	switch t {
	case ReasonTriad:
		return "reason"
	case PlanTriad:
		return "plan"
	case ActTriad:
		return "act"
	case RetrospectTriad:
		return "retrospect"
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
	case IntentAtom, AssessmentAtom, UnderstandingAtom:
		return ReasonTriad
	case PlanAtom, RiskAtom, StrategyAtom:
		return PlanTriad
	case ExecutionAtom, ObservationAtom, AdaptationAtom:
		return ActTriad
	case RetrospectionAtom:
		return RetrospectTriad
	default:
		return ReasonTriad
	}
}

// TriadNodes returns the AtomTypes that belong to a triad.
func TriadNodes(t Triad) []AtomType {
	switch t {
	case ReasonTriad:
		return []AtomType{IntentAtom, AssessmentAtom, UnderstandingAtom}
	case PlanTriad:
		return []AtomType{PlanAtom, RiskAtom, StrategyAtom}
	case ActTriad:
		return []AtomType{ExecutionAtom, ObservationAtom, AdaptationAtom}
	case RetrospectTriad:
		return []AtomType{RetrospectionAtom}
	default:
		return nil
	}
}

// PositionOf returns the dialectic position of an AtomType within its triad.
func PositionOf(t AtomType) DialecticPosition {
	switch t {
	case IntentAtom, PlanAtom, ExecutionAtom:
		return ThesisPosition
	case AssessmentAtom, RiskAtom, ObservationAtom:
		return AntithesisPosition
	case UnderstandingAtom, StrategyAtom, AdaptationAtom:
		return SynthesisPosition
	default:
		return ThesisPosition
	}
}

package reactivity

// Triad groups nodes into Reason, Plan, Act.
type Triad int

const (
	ReasonTriad Triad = iota
	PlanTriad
	ActTriad
)

func (t Triad) String() string {
	switch t {
	case ReasonTriad:
		return "reason"
	case PlanTriad:
		return "plan"
	case ActTriad:
		return "act"
	default:
		return "unknown"
	}
}

// TriadOf returns which triad an AtomType belongs to.
func TriadOf(t AtomType) Triad {
	switch t {
	case IntentAtom, AssessmentAtom:
		return ReasonTriad
	case PlanAtom:
		return PlanTriad
	case ExecutionAtom, RetrospectionAtom:
		return ActTriad
	default:
		return ReasonTriad
	}
}

// TriadNodes returns the AtomTypes that belong to a triad.
func TriadNodes(t Triad) []AtomType {
	switch t {
	case ReasonTriad:
		return []AtomType{IntentAtom, AssessmentAtom}
	case PlanTriad:
		return []AtomType{PlanAtom}
	case ActTriad:
		return []AtomType{ExecutionAtom, RetrospectionAtom}
	default:
		return nil
	}
}

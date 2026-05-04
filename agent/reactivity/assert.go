package reactivity

// YieldKind is the typed outcome of a bond check.
type YieldKind int

const (
	Pass YieldKind = iota
	Insufficient
	Incompatible
	Unresolvable
	Contradiction
)

func (r YieldKind) String() string {
	switch r {
	case Pass:
		return "PASS"
	case Insufficient:
		return "INSUFFICIENT"
	case Incompatible:
		return "INCOMPATIBLE"
	case Unresolvable:
		return "UNRESOLVABLE"
	case Contradiction:
		return "CONTRADICTION"
	default:
		return "UNKNOWN"
	}
}

// Yield is the directive produced by a bond check.
type Yield struct {
	Result  YieldKind
	Message string
	Phase   AtomType
}

// Criticality measures the Molecule's reconciliation momentum.
type Criticality int

const (
	Subcritical   Criticality = iota // momentum dropping — stalling, not enough fuel
	Critical                         // momentum stable — self-sustaining, productive
	Supercritical                    // momentum spiking — rushing, possibly skipping steps
)

func (c Criticality) String() string {
	return [...]string{"subcritical", "critical", "supercritical"}[c]
}

// Assert evaluates a Molecule's criticality based on reconciliation momentum.
type Assert interface {
	Evaluate(m *Molecule) Criticality
}

// AssertFunc adapts a function to the Assert interface.
type AssertFunc func(m *Molecule) Criticality

func (f AssertFunc) Evaluate(m *Molecule) Criticality { return f(m) }

// MomentumAssert evaluates criticality from Momentum() thresholds.
type MomentumAssert struct {
	Low  float64
	High float64
}

// DefaultAssert: below 0.1 transitions/turn = stalling, above 2.0 = rushing.
var DefaultAssert Assert = MomentumAssert{Low: 0.1, High: 2.0}

func (a MomentumAssert) Evaluate(m *Molecule) Criticality {
	if m.Turns() < 2 {
		return Critical
	}
	p := m.Momentum()
	if p < a.Low {
		return Subcritical
	}
	if p > a.High {
		return Supercritical
	}
	return Critical
}

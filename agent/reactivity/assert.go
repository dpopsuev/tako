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

// Yield is the directive produced by Assert.
type Yield struct {
	Result  YieldKind
	Message string
	Phase   AtomType
}

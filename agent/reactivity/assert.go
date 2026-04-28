package reactivity

// AssertResult is the typed outcome of a bond check.
type AssertResult int

const (
	Pass AssertResult = iota
	Insufficient
	Incompatible
	Unresolvable
	Contradiction
)

func (r AssertResult) String() string {
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

// Fortune is the directive produced by Assert.
type Fortune struct {
	Result  AssertResult
	Message string
	Phase   AtomType
}

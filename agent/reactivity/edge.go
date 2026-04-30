package reactivity

// EdgeKind encodes the dialectic relationship between atoms.
type EdgeKind int

const (
	Reference EdgeKind = iota // generic link (backward compat for Atom.Targets)
	Thesis                    // first position in triad — establishes claim
	Antithesis                // contradicts or challenges a thesis
	Synthesis                 // resolves thesis+antithesis into unified view
)

func (k EdgeKind) String() string {
	switch k {
	case Reference:
		return "reference"
	case Thesis:
		return "thesis"
	case Antithesis:
		return "antithesis"
	case Synthesis:
		return "synthesis"
	default:
		return "unknown"
	}
}

// Edge is a typed relationship between two atoms in the Molecule graph.
type Edge struct {
	From string
	To   string
	Kind EdgeKind
}

// Emission is a Motor Bus command queued by a Reactor during processing.
// The Reactor writes emissions to the Molecule. The Cerebrum drains
// and dispatches them via Motor Bus. Zero coupling.
type Emission struct {
	Kind    string
	Target  string
	Payload []byte
}

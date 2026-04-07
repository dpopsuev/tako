// Package topology defines circuit topology primitives and validation.
//
// A topology declares the structural shape of a circuit graph: how many nodes
// it may have, which positions (entry, intermediate, exit) exist, and what
// input/output cardinality each position allows. Circuits declare their
// topology via the topology: field in YAML; the framework validates the
// graph against the declared topology at load time.
//
// Validation is opt-in: circuits without a topology: field skip validation
// and load normally (backward compatible).
//
// # Built-in primitives
//
//	cascade        A ──▶ B ──▶ C          linear pipeline
//	fan-out        A ──▶ B, A ──▶ C       1-to-N scatter
//	fan-in         A ──▶ C, B ──▶ C       N-to-1 gather
//	feedback-loop  A ──▶ B ──▶ C ──back──▶ A   iterative refinement
//	bridge         A1──▶B1, A2──▶B2, B1⇄B2    cross-pollinating paths
//	delegate       A ──▶ D[sub-walk] ──▶ B     meta-circuit via sub-walk
//
// These are composable: a real circuit may be a cascade at the macro level
// with fan-out/fan-in in the middle and a feedback loop on one leg.
package topology

import "fmt"

// Position identifies a node's role within a topology.
type Position string

const (
	PositionEntry        Position = "entry"
	PositionIntermediate Position = "intermediate"
	PositionExit         Position = "exit"
)

// PositionRule defines the allowed input/output cardinality for a position.
// Min/Max of -1 means unbounded (no limit).
type PositionRule struct {
	Position   Position
	MinInputs  int
	MaxInputs  int
	MinOutputs int
	MaxOutputs int
}

// TopologyDef is a named topology primitive with graph-shape constraints.
type TopologyDef struct {
	Name        string
	Description string

	// MinNodes and MaxNodes constrain the total node count.
	// MaxNodes of -1 means unbounded.
	MinNodes int
	MaxNodes int

	// Rules define per-position cardinality constraints.
	Rules []PositionRule
}

// RuleFor returns the PositionRule for the given position, or nil if none.
func (t *TopologyDef) RuleFor(pos Position) *PositionRule {
	for i := range t.Rules {
		if t.Rules[i].Position == pos {
			return &t.Rules[i]
		}
	}
	return nil
}

// Violation is a single validation failure with actionable detail.
type Violation struct {
	NodeName string
	Position Position
	Field    string // "inputs" or "outputs"
	Expected string // e.g. "exactly 1" or "1..3"
	Actual   int
}

func (v Violation) String() string {
	return fmt.Sprintf("node %q at position %q: %s expected %s, got %d",
		v.NodeName, v.Position, v.Field, v.Expected, v.Actual)
}

// ValidationResult holds the outcome of topology validation.
type ValidationResult struct {
	Violations []Violation
}

// OK returns true if there are no violations.
func (r *ValidationResult) OK() bool {
	return len(r.Violations) == 0
}

// Error returns a multi-line error string, or empty if OK.
func (r *ValidationResult) Error() string {
	if r.OK() {
		return ""
	}
	s := fmt.Sprintf("topology validation failed (%d violation(s)):", len(r.Violations))
	for _, v := range r.Violations {
		s += "\n  - " + v.String()
	}
	return s
}

package circuit

// Category: Core Primitives

// Edge is a conditional connection between two Nodes.
// It maps to orchestrate.HeuristicRule: the Evaluate function is the
// edge weight function that determines whether this transition fires.
type Edge interface {
	ID() string
	From() string
	To() string
	IsShortcut() bool
	IsLoop() bool
	Evaluate(artifact Artifact, state *WalkerState) *Transition
}

// ConditionalEdge is an optional interface for edges that have a when: expression.
// Used for structured logging of edge evaluation decisions.
type ConditionalEdge interface {
	Expression() string
}

// ParallelEdge is an optional interface for edges that support fan-out.
// Edges returning true are eligible for concurrent execution when
// multiple parallel edges match from the same source node.
type ParallelEdge interface {
	IsParallel() bool
}

// Transition is the result of evaluating an Edge. It maps to
// orchestrate.HeuristicAction: the next node to visit plus any
// context additions to carry forward.
type Transition struct {
	NextNode         string
	ContextAdditions map[string]any
	Explanation      string
}

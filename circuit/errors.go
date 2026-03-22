package circuit

// Category: Processing & Support

import "errors"

var (
	// ErrNodeNotFound is returned when a referenced node does not exist in the graph.
	ErrNodeNotFound = errors.New("framework: node not found")

	// ErrNoEdge is returned when no edge matches from the current node,
	// indicating the walk has reached a terminal state or a graph definition gap.
	ErrNoEdge = errors.New("framework: no matching edge from node")

	// ErrMaxLoops is returned when a loop edge's counter exceeds the configured maximum.
	ErrMaxLoops = errors.New("framework: max loop iterations exceeded")

	// ErrFanOutMerge is returned when parallel branches disagree on merge target or no merge is found.
	ErrFanOutMerge = errors.New("framework: fan-out merge error")

	// ErrEscalate is returned by RunOperator when Evaluate returns ActionEscalate.
	// The caller (e.g. a Broker) should handle the escalation.
	ErrEscalate = errors.New("framework: operator escalation requested")

	// ErrMaxIterations is returned by RunOperator when the iteration limit is reached
	// without the goal being met.
	ErrMaxIterations = errors.New("framework: operator max iterations exceeded")

	// ErrFindingVeto is returned by VetoHook when a FindingError targets the
	// current node. The hookingWalker intercepts this and wraps the artifact
	// with Confidence() 0.
	ErrFindingVeto = errors.New("framework: finding veto")
)

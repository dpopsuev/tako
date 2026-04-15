package circuit

// Category: Core Primitives

import "context"

// Node is a processing stage in a circuit graph.
// Implementations are domain-specific (e.g. recall, triage, investigate).
type Node interface {
	Name() string
	Approach() Element
	Process(ctx context.Context, nc NodeContext) (Artifact, error)
}

// Artifact is the output of a Node's processing.
// The framework treats it as opaque; typed artifacts are domain-specific.
type Artifact interface {
	Type() string
	Confidence() float64
	Raw() any
}

// CountableArtifact is an optional extension of Artifact for nodes that
// process discrete items. When an artifact implements this interface, the
// walk loop auto-computes signal-to-noise ratio and emits it as "snr"
// metadata on EventNodeExit. Artifacts where item counts don't apply
// (classifications, verdicts, etc.) should not implement this.
type CountableArtifact interface {
	Artifact
	InputCount() int
	OutputCount() int
}

// NodeContext is the input to a Node's Process method: the accumulated
// context for this walker at this node.
type NodeContext struct {
	WalkerState   *WalkerState
	PriorArtifact Artifact
	Meta          map[string]any
}

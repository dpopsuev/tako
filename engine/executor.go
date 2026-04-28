package engine

import (
	"context"

	"github.com/dpopsuev/tako/circuit"
)

// Executor abstracts node execution, decoupling the processing logic from
// its locality.
type Executor interface {
	Execute(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error)
}

// InProcessExecutor runs nodes in the current process by calling
// Node.Process directly. This is the default and zero-allocation path.
type InProcessExecutor struct{}

// Execute delegates to node.Process.
func (InProcessExecutor) Execute(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	return node.Process(ctx, nc)
}

// ExecutorFunc adapts a plain function to the Executor interface.
type ExecutorFunc func(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error)

// Execute calls the wrapped function.
func (f ExecutorFunc) Execute(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	return f(ctx, node, nc)
}

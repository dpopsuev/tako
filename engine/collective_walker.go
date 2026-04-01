package engine

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
)

// CollectiveWalker delegates node processing to one of several inner walkers,
// selected per-node by a WalkerSelector. This replaces the former WalkTeam
// code path — multi-agent scheduling is now inside the walker, not the graph.
type CollectiveWalker struct {
	walkers  []circuit.Walker
	selector WalkerSelector
	observer circuit.WalkObserver
	maxSteps int
	steps    int
	prior    circuit.Walker
}

// CollectiveWalkerOption configures a CollectiveWalker.
type CollectiveWalkerOption func(*CollectiveWalker)

// WithCollectiveObserver attaches a walk observer for walker-switch events.
func WithCollectiveObserver(obs circuit.WalkObserver) CollectiveWalkerOption {
	return func(cw *CollectiveWalker) { cw.observer = obs }
}

// WithMaxSteps sets a defense-in-depth cap on total node visits.
func WithMaxSteps(n int) CollectiveWalkerOption {
	return func(cw *CollectiveWalker) { cw.maxSteps = n }
}

// NewCollectiveWalker creates a walker that delegates per-node to the best
// inner walker as chosen by the selector.
func NewCollectiveWalker(walkers []circuit.Walker, selector WalkerSelector, opts ...CollectiveWalkerOption) *CollectiveWalker {
	cw := &CollectiveWalker{
		walkers:  walkers,
		selector: selector,
	}
	for _, o := range opts {
		o(cw)
	}
	return cw
}

// Identity returns the identity of the primary (first) walker.
func (cw *CollectiveWalker) Identity() circuit.AgentIdentity {
	return cw.walkers[0].Identity()
}

// SetIdentity sets the identity on the primary walker.
func (cw *CollectiveWalker) SetIdentity(id *circuit.AgentIdentity) {
	cw.walkers[0].SetIdentity(id)
}

// State returns the shared walker state (from the primary walker).
func (cw *CollectiveWalker) State() *circuit.WalkerState {
	return cw.walkers[0].State()
}

// Handle selects the best inner walker for this node and delegates processing.
func (cw *CollectiveWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	if cw.maxSteps > 0 {
		cw.steps++
		if cw.steps > cw.maxSteps {
			return nil, ErrMaxStepsExceeded
		}
	}

	selected := cw.selector.SelectWalker(node, cw.walkers, cw.prior)

	if cw.observer != nil && (cw.prior == nil || selected.Identity().Name != cw.prior.Identity().Name) {
		meta := map[string]any{}
		if as, ok := cw.selector.(*AffinitySelector); ok {
			meta["mismatch"] = as.LastMismatch()
		}
		cw.observer.OnEvent(&circuit.WalkEvent{
			Type:     circuit.EventWalkerSwitch,
			Node:     node.Name(),
			Walker:   selected.Identity().Name,
			Metadata: meta,
		})
	}

	cw.prior = selected
	return selected.Handle(ctx, node, nc)
}

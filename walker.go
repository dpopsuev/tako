package framework

// Category: Core Primitives — aliases to core/ package.
// Unexported helpers (trajectoryType, classifyTrajectory, readOnlyContext)
// are duplicated here because root-package code (graph.go, tests) uses them.

import "github.com/dpopsuev/origami/core"

type Walker = core.Walker
type WalkerState = core.WalkerState
type StepRecord = core.StepRecord
type ProcessWalker = core.ProcessWalker

func NewWalkerState(id string) *WalkerState                                   { return core.NewWalkerState(id) }
func NewProcessWalker(id string) *ProcessWalker                               { return core.NewProcessWalker(id) }
func NewProcessWalkerWithIdentity(id AgentIdentity, stateID string) *ProcessWalker { return core.NewProcessWalkerWithIdentity(id, stateID) }

// trajectoryType classifies a confidence convergence pattern.
type trajectoryType string

const (
	TrajectoryUnderdamped      trajectoryType = "underdamped"
	TrajectoryOverdamped       trajectoryType = "overdamped"
	TrajectoryCriticallyDamped trajectoryType = "critically_damped"
	TrajectoryUnstable         trajectoryType = "unstable"
	TrajectoryInsufficient     trajectoryType = "insufficient"
)

// classifyTrajectory analyzes a confidence history to determine the convergence pattern.
func classifyTrajectory(history []float64) trajectoryType {
	if len(history) < 3 {
		return TrajectoryInsufficient
	}

	if history[len(history)-1] < history[0] {
		return TrajectoryUnstable
	}

	signChanges := 0
	prevDelta := history[1] - history[0]
	for i := 2; i < len(history); i++ {
		delta := history[i] - history[i-1]
		if (prevDelta > 0 && delta < 0) || (prevDelta < 0 && delta > 0) {
			signChanges++
		}
		if delta != 0 {
			prevDelta = delta
		}
	}

	switch {
	case signChanges >= 3:
		return TrajectoryUnderdamped
	case signChanges == 0:
		return TrajectoryOverdamped
	default:
		return TrajectoryCriticallyDamped
	}
}

// readOnlyContext returns a shallow copy of the context map.
func readOnlyContext(ctx map[string]any) map[string]any {
	if ctx == nil {
		return nil
	}
	snapshot := make(map[string]any, len(ctx))
	for k, v := range ctx {
		snapshot[k] = v
	}
	return snapshot
}

package framework

// Category: Core Primitives — aliases to core/ package.

import "github.com/dpopsuev/origami/core"

type Walker = core.Walker
type WalkerState = core.WalkerState
type StepRecord = core.StepRecord
type ProcessWalker = core.ProcessWalker

func NewWalkerState(id string) *WalkerState                                        { return core.NewWalkerState(id) }
func NewProcessWalker(id string) *ProcessWalker                                    { return core.NewProcessWalker(id) }
func NewProcessWalkerWithIdentity(id AgentIdentity, stateID string) *ProcessWalker { return core.NewProcessWalkerWithIdentity(id, stateID) }

// trajectoryType classifies a confidence convergence pattern.
type trajectoryType = core.TrajectoryType

const (
	TrajectoryUnderdamped      = core.TrajectoryUnderdamped
	TrajectoryOverdamped       = core.TrajectoryOverdamped
	TrajectoryCriticallyDamped = core.TrajectoryCriticallyDamped
	TrajectoryUnstable         = core.TrajectoryUnstable
	TrajectoryInsufficient     = core.TrajectoryInsufficient
)

// classifyTrajectory analyzes a confidence history to determine the convergence pattern.
func classifyTrajectory(history []float64) trajectoryType {
	return core.ClassifyTrajectory(history)
}

// readOnlyContext returns a shallow copy of the context map.
func readOnlyContext(ctx map[string]any) map[string]any {
	return core.ReadOnlyContext(ctx)
}

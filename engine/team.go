package engine

// Category: Execution

import "github.com/dpopsuev/origami/core"

// Team bundles multiple walkers with scheduling and observability for
// team-based graph traversal.
type Team struct {
	Walkers   []core.Walker
	Scheduler Scheduler
	Observer  core.WalkObserver
	MaxSteps  int // 0 = unlimited
}

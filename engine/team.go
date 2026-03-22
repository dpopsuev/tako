package engine

// Category: Execution

import "github.com/dpopsuev/origami/circuit"

// Team bundles multiple walkers with scheduling and observability for
// team-based graph traversal.
type Team struct {
	Walkers   []circuit.Walker
	Scheduler Scheduler
	Observer  circuit.WalkObserver
	MaxSteps  int // 0 = unlimited
}

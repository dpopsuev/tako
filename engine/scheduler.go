package engine

// Category: Execution

import "github.com/dpopsuev/origami/core"

// SchedulerContext provides all information a Scheduler needs to pick a walker.
type SchedulerContext struct {
	Node        core.Node
	Zone        *Zone
	Walkers     []core.Walker
	PriorWalker core.Walker
	WalkState   *core.WalkerState
}

// Scheduler selects which walker handles a given node.
type Scheduler interface {
	Select(ctx SchedulerContext) core.Walker
}

// SingleScheduler always returns the same walker.
type SingleScheduler struct {
	Walker core.Walker
}

func (s *SingleScheduler) Select(_ SchedulerContext) core.Walker {
	return s.Walker
}

// AffinityScheduler picks the walker whose StepAffinity for the node
// is highest.
type AffinityScheduler struct {
	lastMismatch float64
}

// LastMismatch returns the impedance mismatch from the most recent Select.
func (s *AffinityScheduler) LastMismatch() float64 {
	return s.lastMismatch
}

func (s *AffinityScheduler) Select(ctx SchedulerContext) core.Walker {
	s.lastMismatch = 1.0

	if len(ctx.Walkers) == 0 {
		return nil
	}
	if len(ctx.Walkers) == 1 {
		s.lastMismatch = computeMismatch(ctx.Walkers[0], ctx.Node)
		return ctx.Walkers[0]
	}

	nodeName := ctx.Node.Name()
	nodeElement := ctx.Node.ElementAffinity()

	var best core.Walker
	bestScore := -1.0
	bestElementMatch := false

	for _, w := range ctx.Walkers {
		id := w.Identity()
		score := id.StepAffinity[nodeName]
		elementMatch := id.Element == nodeElement

		better := false
		switch {
		case score > bestScore:
			better = true
		case score == bestScore && elementMatch && !bestElementMatch:
			better = true
		}

		if better {
			best = w
			bestScore = score
			bestElementMatch = elementMatch
		}
	}

	if best == nil {
		s.lastMismatch = 1.0
		return ctx.Walkers[0]
	}
	s.lastMismatch = computeMismatch(best, ctx.Node)
	return best
}

// computeMismatch scores how well a walker fits a node (0 = perfect, 1 = worst).
func computeMismatch(w core.Walker, node core.Node) float64 {
	id := w.Identity()
	affinityScore := id.StepAffinity[node.Name()]
	elementBonus := 0.0
	if id.Element == node.ElementAffinity() && id.Element != "" {
		elementBonus = 0.5
	}
	maxPossible := 1.5
	return 1.0 - (affinityScore+elementBonus)/maxPossible
}

// zoneForNode finds the zone containing a node, or nil.
func zoneForNode(nodeName string, zones []Zone) *Zone {
	for i := range zones {
		for _, n := range zones[i].NodeNames {
			if n == nodeName {
				return &zones[i]
			}
		}
	}
	return nil
}

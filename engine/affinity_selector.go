package engine

import "github.com/dpopsuev/tako/circuit"

// AffinitySelector picks the walker whose StepAffinity for the current node
// is highest, with Approach as a tiebreaker.
type AffinitySelector struct {
	lastMismatch float64
}

// LastMismatch returns the impedance mismatch from the most recent selection.
func (s *AffinitySelector) LastMismatch() float64 {
	return s.lastMismatch
}

func (s *AffinitySelector) SelectWalker(node circuit.Node, walkers []circuit.Walker, _ circuit.Walker) circuit.Walker {
	s.lastMismatch = 1.0

	if len(walkers) == 0 {
		return nil
	}
	if len(walkers) == 1 {
		s.lastMismatch = affinityMismatch(walkers[0], node)
		return walkers[0]
	}

	nodeName := node.Name()
	nodeElement := node.Approach()

	var best circuit.Walker
	bestScore := -1.0
	bestElementMatch := false

	for _, w := range walkers {
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
		return walkers[0]
	}
	s.lastMismatch = affinityMismatch(best, node)
	return best
}

func affinityMismatch(w circuit.Walker, node circuit.Node) float64 {
	id := w.Identity()
	affinityScore := id.StepAffinity[node.Name()]
	elementBonus := 0.0
	if id.Element == node.Approach() && id.Element != "" {
		elementBonus = 0.5
	}
	maxPossible := 1.5
	return 1.0 - (affinityScore+elementBonus)/maxPossible
}

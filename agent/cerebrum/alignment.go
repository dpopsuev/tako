package cerebrum

import (
	"github.com/dpopsuev/tako/agent/reactivity"
)

type AlignmentTier int

const (
	TierStructural  AlignmentTier = iota // T0: residual delta — did distance change?
	TierDimensional                      // T1: atom dimensions ∩ unmet residual
	TierTrajectory                       // T2: sequence matches book move (future)
	TierSemantic                         // T3: LLM-as-judge (future)
)

type AlignmentResult struct {
	Tier       AlignmentTier
	Score      float64
	Aligned    bool
	DriftFlags []string
}

type AlignmentChecker interface {
	Check(atom reactivity.Atom, m *reactivity.Molecule) AlignmentResult
}

type TieredAlignment struct{}

func (TieredAlignment) Check(atom reactivity.Atom, m *reactivity.Molecule) AlignmentResult {
	t0 := checkStructural(m)
	if !t0.Aligned {
		return t0
	}
	return checkDimensional(atom, m)
}

func checkStructural(m *reactivity.Molecule) AlignmentResult {
	delta := m.DeltaDistance()
	if m.Turns() < 2 {
		return AlignmentResult{Tier: TierStructural, Score: 1, Aligned: true}
	}
	if delta > 0 {
		return AlignmentResult{
			Tier:       TierStructural,
			Score:      0,
			Aligned:    false,
			DriftFlags: []string{"distance_regressing"},
		}
	}
	score := 1.0
	if delta == 0 {
		score = 0.5
	}
	return AlignmentResult{Tier: TierStructural, Score: score, Aligned: true}
}

func checkDimensional(atom reactivity.Atom, m *reactivity.Molecule) AlignmentResult {
	residual := m.Residual()
	if residual == nil || len(atom.Dimensions) == 0 {
		return AlignmentResult{
			Tier:       TierDimensional,
			Score:      0.5,
			Aligned:    true,
			DriftFlags: noDimensionFlags(atom, residual),
		}
	}

	unmet := 0
	for _, v := range residual {
		if v > 0 {
			unmet++
		}
	}
	if unmet == 0 {
		return AlignmentResult{Tier: TierDimensional, Score: 1, Aligned: true}
	}

	overlap := 0
	wasted := 0
	for _, dim := range atom.Dimensions {
		if v, ok := residual[dim]; ok {
			if v > 0 {
				overlap++
			} else {
				wasted++
			}
		}
	}

	var flags []string
	if wasted > 0 {
		flags = append(flags, "wasted_dimensions")
	}
	if overlap == 0 && len(atom.Dimensions) > 0 {
		flags = append(flags, "no_unmet_overlap")
	}

	score := float64(overlap) / float64(len(atom.Dimensions))
	return AlignmentResult{
		Tier:       TierDimensional,
		Score:      score,
		Aligned:    overlap > 0 || len(atom.Dimensions) == 0,
		DriftFlags: flags,
	}
}

func noDimensionFlags(atom reactivity.Atom, residual map[string]float64) []string {
	if len(atom.Dimensions) == 0 && residual != nil {
		return []string{"no_dimensions_claimed"}
	}
	return nil
}

// EXPERIMENTAL: ConvergenceAssert wraps MomentumAssert — ablation needed (TSK-436)
type ConvergenceAssert struct {
	Inner          reactivity.Assert
	StagnantLimit  int
}

func (a ConvergenceAssert) Evaluate(m *reactivity.Molecule) reactivity.Criticality {
	base := a.Inner.Evaluate(m)
	if base == reactivity.Subcritical {
		return base
	}

	triad := m.CurrentTriad()
	diff := m.SynthesisDiff(triad)

	if diff == 0 && m.Distance() > 0 && m.DeltaDistance() >= 0 {
		return reactivity.Subcritical
	}

	return base
}

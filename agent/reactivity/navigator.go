package reactivity

import "log/slog"

// Navigator decides which Sephirah the Molecule flows to next.
// Called after EVERY phase node, not just after Triad seals.
// Returns the next AtomType to transition to, or the same current
// phase to continue within the Triad normally.
type Navigator func(m *Molecule, current AtomType) AtomType

// LinearNavigator always follows the fixed sequence within each Triad.
// Only makes inter-Triad decisions: Think → Compose → Implement → Reflect.
// Returns current.Next() for intra-Triad progression (no skipping).
var LinearNavigator Navigator = func(m *Molecule, current AtomType) AtomType {
	next := linearNext(current)
	if next != current {
		slog.Info("navigator.decision",
			slog.String("navigator", "linear"),
			slog.String("from", current.String()),
			slog.String("next", next.String()),
			slog.String("molecule", m.ID))
	}
	return next
}

func linearNext(current AtomType) AtomType {
	switch current {
	// Think triad: thesis → antithesis → synthesis
	case IntentAtom:
		return AssessmentAtom
	case AssessmentAtom:
		return KnowledgeAtom
	// Think synthesis → Compose thesis
	case KnowledgeAtom:
		return ExpansionAtom
	// Compose triad
	case ExpansionAtom:
		return ReductionAtom
	case ReductionAtom:
		return SelectionAtom
	// Compose synthesis → Implement thesis
	case SelectionAtom:
		return ExecutionAtom
	// Implement triad
	case ExecutionAtom:
		return AcclimationAtom
	case AcclimationAtom:
		return RefinementAtom
	// Implement synthesis → Reflect
	case RefinementAtom:
		return RetrospectionAtom
	default:
		return RetrospectionAtom
	}
}

// TreeNavigator navigates the Tree of Life per-Sephirah.
// At each node, evaluates distance + recollection to decide:
// continue linearly, skip ahead, or shortcut to Execution.
// TreeNavigator uses PID control on the residual tensor:
//   P (distance): how far from Desired
//   I (momentum): accumulated phase transitions / turns
//   D (delta): rate of distance change per turn
// Combined with Capability.Writes as MPC prediction model
// and Book Moves as feedforward.
var TreeNavigator Navigator = func(m *Molecule, current AtomType) AtomType {
	p := m.Distance()     // P: proportional — how far
	d := m.DeltaDistance() // D: derivative — rate of change
	recollected := m.SourceMass(Recollected)
	total := m.TotalMass()
	ratio := float64(0)
	if total > 0 {
		ratio = float64(recollected) / float64(total)
	}
	residual := m.Residual()
	unmetCount := 0
	for _, v := range residual {
		if v > 0 {
			unmetCount++
		}
	}

	var next AtomType
	var reason string

	switch current {
	case IntentAtom:
		if ratio > 0.3 && p < 0.3 {
			next = ExecutionAtom
			reason = "intent→execution: feedforward (recollection>0.3 + P<0.3)"
		} else if p < 0.3 {
			next = SelectionAtom
			reason = "intent→selection: P<0.3, skip deliberation"
		} else if unmetCount == 1 {
			next = SelectionAtom
			reason = "intent→selection: single unmet dimension, skip to planning"
		} else {
			next = AssessmentAtom
			reason = "intent→assessment: P>=0.3, multiple unmet dimensions"
		}

	case AssessmentAtom:
		if p < 0.5 || unmetCount <= 2 {
			next = SelectionAtom
			reason = "assessment→selection: P<0.5 or <=2 unmet dimensions"
		} else {
			next = KnowledgeAtom
			reason = "assessment→knowledge: P>=0.5 and >2 unmet, need depth"
		}

	case KnowledgeAtom:
		if p < 0.5 {
			next = SelectionAtom
			reason = "knowledge→selection: P<0.5, sufficient for planning"
		} else {
			next = ExpansionAtom
			reason = "knowledge→expansion: P>=0.5, explore options"
		}

	case ExpansionAtom:
		next = ReductionAtom
		reason = "expansion→reduction: filter options"

	case ReductionAtom:
		next = SelectionAtom
		reason = "reduction→selection: commit to plan"

	case SelectionAtom:
		next = ExecutionAtom
		reason = "selection→execution: execute the plan"

	case ExecutionAtom:
		next = AcclimationAtom
		reason = "execution→acclimation: observe results"

	case AcclimationAtom:
		if p == 0 {
			next = RetrospectionAtom
			reason = "acclimation→retrospection: P=0, goal reached"
		} else if d > 0 && m.Turns() > 3 {
			next = RetrospectionAtom
			reason = "acclimation→retrospection: D>0 (going backward), cut losses"
		} else {
			next = RefinementAtom
			reason = "acclimation→refinement: P>0, refine approach"
		}

	case RefinementAtom:
		next = RetrospectionAtom
		reason = "refinement→retrospection: seal"

	default:
		next = RetrospectionAtom
		reason = "default→retrospection"
	}

	if next != linearNext(current) {
		slog.Info("navigator.shortcut",
			slog.String("navigator", "tree"),
			slog.String("from", current.String()),
			slog.String("next", next.String()),
			slog.String("linear_would", linearNext(current).String()),
			slog.Float64("P", p),
			slog.Float64("D", d),
			slog.Float64("recollection", ratio),
			slog.Int("unmet", unmetCount),
			slog.Any("residual", residual),
			slog.String("reason", reason),
			slog.String("molecule", m.ID))
	} else {
		slog.Debug("navigator.decision",
			slog.String("navigator", "tree"),
			slog.String("from", current.String()),
			slog.String("next", next.String()),
			slog.Float64("P", p),
			slog.Float64("D", d),
			slog.Int("unmet", unmetCount),
			slog.String("reason", reason),
			slog.String("molecule", m.ID))
	}

	return next
}

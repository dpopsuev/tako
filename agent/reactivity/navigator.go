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
var TreeNavigator Navigator = func(m *Molecule, current AtomType) AtomType {
	d := m.Distance()
	recollected := m.SourceMass(Recollected)
	total := m.TotalMass()
	ratio := float64(0)
	if total > 0 {
		ratio = float64(recollected) / float64(total)
	}

	var next AtomType
	var reason string

	switch current {
	// After Intent: can we shortcut?
	case IntentAtom:
		if ratio > 0.3 && d < 0.3 {
			next = ExecutionAtom
			reason = "intent→execution: recollection>0.3 + distance<0.3, known territory"
		} else if d < 0.3 {
			next = SelectionAtom
			reason = "intent→selection: distance<0.3, skip deliberation"
		} else {
			next = AssessmentAtom
			reason = "intent→assessment: need assessment"
		}

	// After Assessment: skip to Selection or continue?
	case AssessmentAtom:
		if d < 0.5 {
			next = SelectionAtom
			reason = "assessment→selection: distance<0.5 after assessment, skip to selection"
		} else {
			next = KnowledgeAtom
			reason = "assessment→knowledge: need deeper knowledge"
		}

	// After Knowledge: go to Expansion or skip to Selection?
	case KnowledgeAtom:
		if d < 0.5 {
			next = SelectionAtom
			reason = "knowledge→selection: distance<0.5, knowledge sufficient for selection"
		} else {
			next = ExpansionAtom
			reason = "knowledge→expansion: need to explore options"
		}

	// After Expansion: always Reduction (expansion→reduction)
	case ExpansionAtom:
		next = ReductionAtom
		reason = "expansion→reduction: filter options"

	// After Reduction: always Selection (reduction→selection)
	case ReductionAtom:
		next = SelectionAtom
		reason = "reduction→selection: commit to plan"

	// After Selection: always Execution (selection→execution)
	case SelectionAtom:
		next = ExecutionAtom
		reason = "selection→execution: execute the plan"

	// After Execution: always Acclimation (execution→acclimation)
	case ExecutionAtom:
		next = AcclimationAtom
		reason = "execution→acclimation: observe results"

	// After Acclimation: skip Refinement if distance is 0?
	case AcclimationAtom:
		if d == 0 {
			next = RetrospectionAtom
			reason = "acclimation→retrospection: distance=0, skip refinement"
		} else {
			next = RefinementAtom
			reason = "acclimation→refinement: refine approach"
		}

	// After Refinement: always Retrospection (refinement→retrospection)
	case RefinementAtom:
		next = RetrospectionAtom
		reason = "refinement→retrospection: seal"

	default:
		next = RetrospectionAtom
		reason = "default→retrospection"
	}

	residual := m.Residual()
	if next != linearNext(current) {
		slog.Info("navigator.shortcut",
			slog.String("navigator", "tree"),
			slog.String("from", current.String()),
			slog.String("next", next.String()),
			slog.String("linear_would", linearNext(current).String()),
			slog.Float64("distance", d),
			slog.Float64("recollection_ratio", ratio),
			slog.Any("residual", residual),
			slog.String("reason", reason),
			slog.String("molecule", m.ID))
	} else {
		slog.Debug("navigator.decision",
			slog.String("navigator", "tree"),
			slog.String("from", current.String()),
			slog.String("next", next.String()),
			slog.Float64("distance", d),
			slog.Any("residual", residual),
			slog.String("reason", reason),
			slog.String("molecule", m.ID))
	}

	return next
}

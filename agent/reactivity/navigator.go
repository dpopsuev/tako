package reactivity

import "log/slog"

// Navigator decides which Sephirah the Molecule flows to next
// after a Triad seals. The Tree of Life has 22 valid paths —
// the Navigator picks which one to take from the current node.
type Navigator func(m *Molecule, sealed Triad) AtomType

// LinearNavigator always follows the fixed sequence:
// Think → Compose → Implement → Reflect.
var LinearNavigator Navigator = func(m *Molecule, sealed Triad) AtomType {
	var next AtomType
	switch sealed {
	case ThinkTriad:
		next = ExpansionAtom
	case ComposeTriad:
		next = ExecutionAtom
	case ImplementTriad:
		next = RetrospectionAtom
	default:
		next = RetrospectionAtom
	}
	slog.Info("navigator.decision",
		slog.String("navigator", "linear"),
		slog.String("sealed", sealed.String()),
		slog.String("next", next.String()),
		slog.String("molecule", m.ID))
	return next
}

// TreeNavigator uses the Molecule's distance to Desired state
// to select paths on the Tree of Life. Short distance = shortcuts.
// Long distance = full deliberation.
var TreeNavigator Navigator = func(m *Molecule, sealed Triad) AtomType {
	d := m.Distance()
	recollected := m.SourceMass(Recollected)
	total := m.TotalMass()
	ratio := float64(0)
	if total > 0 {
		ratio = float64(recollected) / float64(total)
	}
	momentum := m.Momentum()
	turns := m.Turns()

	var next AtomType
	var reason string

	switch sealed {
	case ThinkTriad:
		if ratio > 0.3 && d < 0.3 {
			next = ExecutionAtom
			reason = "recollection>0.3 + distance<0.3: known territory, skip to execution"
		} else if d < 0.3 {
			next = SelectionAtom
			reason = "distance<0.3: close to goal, skip expand/reduce"
		} else if d < 0.6 {
			next = SelectionAtom
			reason = "distance<0.6: moderate gap, skip expand/reduce"
		} else {
			next = ExpansionAtom
			reason = "distance>=0.6: far from goal, full compose path"
		}

	case ComposeTriad:
		next = ExecutionAtom
		reason = "compose sealed: execute the plan"

	case ImplementTriad:
		next = RetrospectionAtom
		reason = "implement sealed: reflect on results"

	default:
		next = RetrospectionAtom
		reason = "default: retrospection"
	}

	slog.Info("navigator.decision",
		slog.String("navigator", "tree"),
		slog.String("sealed", sealed.String()),
		slog.String("next", next.String()),
		slog.Float64("distance", d),
		slog.Float64("recollection_ratio", ratio),
		slog.Float64("momentum", momentum),
		slog.Int("turns", turns),
		slog.Int("mass", total),
		slog.Int("recollected", recollected),
		slog.String("reason", reason),
		slog.String("molecule", m.ID))

	return next
}

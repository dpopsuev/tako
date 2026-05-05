package reactivity

// Navigator decides which Sephirah the Molecule flows to next
// after a Triad seals. The Tree of Life has 22 valid paths —
// the Navigator picks which one to take from the current node.
type Navigator func(m *Molecule, sealed Triad) AtomType

// LinearNavigator always follows the fixed sequence:
// Think → Compose → Implement → Reflect.
// This is the current default behavior.
var LinearNavigator Navigator = func(_ *Molecule, sealed Triad) AtomType {
	switch sealed {
	case ThinkTriad:
		return ExpansionAtom
	case ComposeTriad:
		return ExecutionAtom
	case ImplementTriad:
		return RetrospectionAtom
	default:
		return RetrospectionAtom
	}
}

// TreeNavigator uses the Molecule's distance to Desired state
// to select paths on the Tree of Life. Short distance = shortcuts.
// Long distance = full deliberation.
//
// Paths (Sephirot → Sephirot):
//   Think sealed:
//     distance < 0.3 → Execution (shortcut: plan is known)
//     distance < 0.6 → Selection (skip expand/reduce, commit directly)
//     distance >= 0.6 → Expansion (full compose path)
//   Compose sealed:
//     always → Execution (plan is made, execute it)
//   Implement sealed:
//     always → Retrospection (reflect on results)
var TreeNavigator Navigator = func(m *Molecule, sealed Triad) AtomType {
	switch sealed {
	case ThinkTriad:
		d := m.Distance()
		recollected := m.SourceMass(Recollected)
		total := m.TotalMass()

		// High recollection ratio + low distance = known territory
		if total > 0 && float64(recollected)/float64(total) > 0.3 && d < 0.3 {
			return ExecutionAtom
		}
		// Low distance = close to goal, skip deliberation
		if d < 0.3 {
			return SelectionAtom
		}
		// Medium distance = need a plan but not deep exploration
		if d < 0.6 {
			return SelectionAtom
		}
		// High distance = full compose path
		return ExpansionAtom

	case ComposeTriad:
		return ExecutionAtom

	case ImplementTriad:
		return RetrospectionAtom

	default:
		return RetrospectionAtom
	}
}

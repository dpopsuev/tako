package reactivity

import (
	"testing"
)

func TestExit_WishFromReason(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("unreasonable")
	c.Add(m, mkAtom("desire-fly", IntentAtom, "intent.desire.fly", Fresh))
	c.Add(m, mkAtom("assess-no-wings", AssessmentAtom, "assessment.state.no-wings", Fresh))

	c.Seal(m, mkAtom("wish-need-wings", RetrospectionAtom, "retrospection.wish.need-wings", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after wish from reason")
	}
	if m.TotalMass() != 3 {
		t.Errorf("should preserve 3 atoms (intent + assessment + wish), got %d", m.TotalMass())
	}
}

func TestExit_WishFromFormation(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("no-path")
	addReasonAtoms(c, m, "eat")
	c.Add(m, mkAtom("opt-cook", ExpansionAtom, "plan.option.cook", Fresh))
	c.Add(m, mkAtom("risk-burn", ReductionAtom, "risk.eval.burn", Fresh))

	c.Seal(m, mkAtom("wish-buy-food", RetrospectionAtom, "retrospection.wish.buy-food", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after wish from formation")
	}
}

func TestExit_WishFromAction(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("unrecoverable")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	c.Add(m, mkAtom("exec-fail", ExecutionAtom, "execution.result.broom-broke", Fresh))

	c.Seal(m, mkAtom("wish-new-broom", RetrospectionAtom, "retrospection.wish.need-broom", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after wish from action")
	}
}

func TestExit_SealedRejectsNewAtoms(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("done")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))

	result, _ := c.Add(m, mkAtom("late", AssessmentAtom, "assessment.state.late", Fresh))
	if result != Unresolvable {
		t.Errorf("sealed molecule should reject atoms with Unresolvable, got %s", result)
	}
}

func TestExit_DraftNotSealed(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("draft")
	addReasonAtoms(c, m, "eat")
	c.Add(m, mkAtom("plan", ExpansionAtom, "plan.option.cook", Fresh))

	if m.Sealed() {
		t.Error("molecule with plan but no wish should not be sealed")
	}
	if m.TotalMass() != 4 {
		t.Errorf("expected 4 atoms in draft, got %d", m.TotalMass())
	}
}

func TestExit_AbortPreservesPartialWork(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("abort")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	c.Add(m, mkAtom("exec1", ExecutionAtom, "execution.result.bed", Fresh))

	c.Seal(m, mkAtom("wish-budget", RetrospectionAtom, "retrospection.wish.budget-exceeded", Fresh))

	if !m.Sealed() {
		t.Error("aborted molecule should be sealed")
	}
	if m.Mass(ExecutionAtom) != 1 {
		t.Errorf("should preserve partial execution, got %d", m.Mass(ExecutionAtom))
	}
	if m.Mass(SelectionAtom) != 1 {
		t.Errorf("should preserve strategy from formation, got %d", m.Mass(SelectionAtom))
	}
}

func TestExit_TerminateRecovery(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("crash")
	addReasonAtoms(c, m, "clean")
	c.Add(m, mkAtom("task", ExpansionAtom, "plan.task.sweep", Fresh))

	if m.Sealed() {
		t.Error("crashed molecule is NOT sealed (nobody called Seal)")
	}
	if m.TotalMass() != 4 {
		t.Errorf("atoms should be preserved after crash, got %d", m.TotalMass())
	}
}

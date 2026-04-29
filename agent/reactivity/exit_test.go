package reactivity

import (
	"testing"
	"time"
)

func TestExit_WishFromReason(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("unreasonable")
	c.Add(m, mkAtom("desire-fly", IntentAtom, "intent.desire.fly", Fresh))
	c.Add(m, mkAtom("assess-no-wings", AssessmentAtom, "assessment.state.no-wings", Fresh))

	c.Seal(m, mkAtom("wish-need-wings", RetrospectionAtom, "retrospection.wish.need-wings", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after Wish from Reason")
	}
	if m.TotalMass() != 3 {
		t.Errorf("should preserve 3 atoms (desire + assessment + wish), got %d", m.TotalMass())
	}
}

func TestExit_WishFromPlan(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("no-path")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("assess-empty", AssessmentAtom, "assessment.state.fridge-empty", Fresh))
	c.Add(m, mkAtom("opt-cook", PlanAtom, "plan.option.cook", Fresh))

	c.Seal(m, mkAtom("wish-buy-food", RetrospectionAtom, "retrospection.wish.buy-food", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after Wish from Plan")
	}
}

func TestExit_WishFromAct(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("unrecoverable")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("exec-fail", ExecutionAtom, "execution.result.broom-broke", Fresh))

	c.Seal(m, mkAtom("wish-new-broom", RetrospectionAtom, "retrospection.wish.need-broom", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after Wish from Act")
	}
}

func TestExit_SealedRejectsNewAtoms(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("done")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))

	result, _ := c.Add(m, mkAtom("late", AssessmentAtom, "assessment.state.late", Fresh))
	if result != Unresolvable {
		t.Errorf("sealed circuit should reject atoms with Unresolvable, got %s", result)
	}
}

func TestExit_DraftNotSealed(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("draft")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("assess", AssessmentAtom, "assessment.availability.fridge", Fresh))
	c.Add(m, mkAtom("plan", PlanAtom, "plan.option.cook", Fresh))

	if m.Sealed() {
		t.Error("circuit with plan but no Wish should not be sealed")
	}
	if m.TotalMass() != 3 {
		t.Errorf("expected 3 atoms in draft, got %d", m.TotalMass())
	}
}

func TestExit_AbortPreservesPartialWork(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("abort")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task1", PlanAtom, "plan.task.bed", Fresh))
	c.Add(m, mkAtom("task2", PlanAtom, "plan.task.floor", Fresh))
	c.Add(m, mkAtom("exec1", ExecutionAtom, "execution.result.bed", Fresh, "task1"))

	c.Seal(m, mkAtom("wish-budget", RetrospectionAtom, "retrospection.wish.budget-exceeded", Fresh))

	if !m.Sealed() {
		t.Error("aborted circuit should be sealed")
	}
	if m.Mass(ExecutionAtom) != 1 {
		t.Errorf("should preserve partial execution (1 of 2 tasks), got %d", m.Mass(ExecutionAtom))
	}
	if m.Mass(PlanAtom) != 2 {
		t.Errorf("should preserve full plan (2 tasks), got %d", m.Mass(PlanAtom))
	}
}

func TestExit_TerminateRecovery(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("crash")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	if m.Sealed() {
		t.Error("crashed circuit is NOT sealed (nobody called Seal)")
	}
	if m.TotalMass() != 3 {
		t.Errorf("atoms should be preserved after crash, got %d", m.TotalMass())
	}

	_ = time.Now()
}

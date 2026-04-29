package reactivity

import (
	"testing"
	"time"
)

func TestExit_WishFromReason(t *testing.T) {
	c := NewCircuit("unreasonable")
	c.Add(mkAtom("desire-fly", IntentAtom, "intent.desire.fly", Fresh))
	c.Add(mkAtom("assess-no-wings", AssessmentAtom, "assessment.state.no-wings", Fresh))

	c.Seal(mkAtom("wish-need-wings", RetrospectionAtom, "retrospection.wish.need-wings", Fresh))

	if !c.Sealed() {
		t.Error("should be sealed after Wish from Reason")
	}
	if c.TotalMass() != 3 {
		t.Errorf("should preserve 3 atoms (desire + assessment + wish), got %d", c.TotalMass())
	}
}

func TestExit_WishFromPlan(t *testing.T) {
	c := NewCircuit("no-path")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("assess-empty", AssessmentAtom, "assessment.state.fridge-empty", Fresh))
	c.Add(mkAtom("opt-cook", PlanAtom, "plan.option.cook", Fresh))

	c.Seal(mkAtom("wish-buy-food", RetrospectionAtom, "retrospection.wish.buy-food", Fresh))

	if !c.Sealed() {
		t.Error("should be sealed after Wish from Plan")
	}
}

func TestExit_WishFromAct(t *testing.T) {
	c := NewCircuit("unrecoverable")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(mkAtom("exec-fail", ExecutionAtom, "execution.result.broom-broke", Fresh))

	c.Seal(mkAtom("wish-new-broom", RetrospectionAtom, "retrospection.wish.need-broom", Fresh))

	if !c.Sealed() {
		t.Error("should be sealed after Wish from Act")
	}
}

func TestExit_SealedRejectsNewAtoms(t *testing.T) {
	c := NewCircuit("done")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Seal(mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))

	result, _ := c.Add(mkAtom("late", AssessmentAtom, "assessment.state.late", Fresh))
	if result != Unresolvable {
		t.Errorf("sealed circuit should reject atoms with Unresolvable, got %s", result)
	}
}

func TestExit_DraftNotSealed(t *testing.T) {
	c := NewCircuit("draft")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("assess", AssessmentAtom, "assessment.availability.fridge", Fresh))
	c.Add(mkAtom("plan", PlanAtom, "plan.option.cook", Fresh))

	if c.Sealed() {
		t.Error("circuit with plan but no Wish should not be sealed")
	}
	if c.TotalMass() != 3 {
		t.Errorf("expected 3 atoms in draft, got %d", c.TotalMass())
	}
}

func TestExit_AbortPreservesPartialWork(t *testing.T) {
	c := NewCircuit("abort")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task1", PlanAtom, "plan.task.bed", Fresh))
	c.Add(mkAtom("task2", PlanAtom, "plan.task.floor", Fresh))
	c.Add(mkAtom("exec1", ExecutionAtom, "execution.result.bed", Fresh, "task1"))

	c.Seal(mkAtom("wish-budget", RetrospectionAtom, "retrospection.wish.budget-exceeded", Fresh))

	if !c.Sealed() {
		t.Error("aborted circuit should be sealed")
	}
	if c.Mass(ExecutionAtom) != 1 {
		t.Errorf("should preserve partial execution (1 of 2 tasks), got %d", c.Mass(ExecutionAtom))
	}
	if c.Mass(PlanAtom) != 2 {
		t.Errorf("should preserve full plan (2 tasks), got %d", c.Mass(PlanAtom))
	}
}

func TestExit_TerminateRecovery(t *testing.T) {
	c := NewCircuit("crash")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("assess", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	if c.Sealed() {
		t.Error("crashed circuit is NOT sealed (nobody called Seal)")
	}
	if c.TotalMass() != 3 {
		t.Errorf("atoms should be preserved after crash, got %d", c.TotalMass())
	}

	_ = time.Now()
}

package reactivity

import "testing"

func TestCascade_UnsealPlanDoesNotUnsealReason(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	if !c.TriadSealed(ReasonTriad) {
		t.Fatal("Reason should be sealed")
	}
	if !c.TriadSealed(PlanTriad) {
		t.Fatal("Plan should be sealed")
	}

	c.UnsealTriad(PlanTriad)

	if !c.TriadSealed(ReasonTriad) {
		t.Error("Reason should STAY sealed when Plan unseals (North Star fixed)")
	}
	if c.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed")
	}
	if c.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed (cascade down from Plan)")
	}
}

func TestCascade_UnsealReasonCascadesAll(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(mkAtom("retro", RetrospectionAtom, "retrospection.learning.done", Fresh))

	if !c.AllTriadsSealed() {
		t.Fatal("all triads should be sealed after full chain")
	}

	c.UnsealTriad(ReasonTriad)

	if c.TriadSealed(ReasonTriad) {
		t.Error("Reason should be unsealed")
	}
	if c.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed (cascade from Reason)")
	}
	if c.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed (cascade from Reason)")
	}
}

func TestCascade_UnsealActOnlyAffectsAct(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(mkAtom("retro", RetrospectionAtom, "retrospection.learning.done", Fresh))

	c.UnsealTriad(ActTriad)

	if !c.TriadSealed(ReasonTriad) {
		t.Error("Reason should stay sealed")
	}
	if !c.TriadSealed(PlanTriad) {
		t.Error("Plan should stay sealed")
	}
	if c.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed")
	}
}

func TestCascade_AdaptNeverUnsealsReason(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.floor", Fresh))
	c.Add(mkAtom("done", ExecutionAtom, "execution.result.floor", Fresh))

	dirty := mkAtom("dirty-again", AssessmentAtom, "assessment.state.floor", Fresh)
	contradicts, _ := c.Contradict(dirty)
	if contradicts {
		c.UnsealTriad(PlanTriad)
	}

	if !c.TriadSealed(ReasonTriad) {
		t.Error("Adapt (via contradiction) should unseal Plan but NEVER unseal Reason")
	}
	if c.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed after Adapt contradiction")
	}
}

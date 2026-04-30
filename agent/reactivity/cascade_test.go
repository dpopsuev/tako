package reactivity

import "testing"

func TestCascade_UnsealPlanDoesNotUnsealReason(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	if !m.TriadSealed(ReasonTriad) {
		t.Fatal("Reason should be sealed")
	}
	if !m.TriadSealed(PlanTriad) {
		t.Fatal("Plan should be sealed")
	}

	c.UnsealTriad(m, PlanTriad)

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Reason should STAY sealed when Plan unseals (North Star fixed)")
	}
	if m.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed")
	}
	if m.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed (cascade down from Plan)")
	}
}

func TestCascade_UnsealReasonCascadesAll(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("retro", RetrospectionAtom, "retrospection.learning.done", Fresh))

	if !m.AllTriadsSealed() {
		t.Fatal("all triads should be sealed after full chain")
	}

	c.UnsealTriad(m, ReasonTriad)

	if m.TriadSealed(ReasonTriad) {
		t.Error("Reason should be unsealed")
	}
	if m.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed (cascade from Reason)")
	}
	if m.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed (cascade from Reason)")
	}
}

func TestCascade_UnsealActOnlyAffectsAct(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("retro", RetrospectionAtom, "retrospection.learning.done", Fresh))

	c.UnsealTriad(m, ActTriad)

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Reason should stay sealed")
	}
	if !m.TriadSealed(PlanTriad) {
		t.Error("Plan should stay sealed")
	}
	if m.TriadSealed(ActTriad) {
		t.Error("Act should be unsealed")
	}
}

func TestCascade_AdaptNeverUnsealsReason(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.floor", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.floor", Fresh))

	dirty := mkAtom("dirty-again", AssessmentAtom, "assessment.state.floor", Fresh)
	contradicts, _ := c.Contradict(m, dirty)
	if contradicts {
		c.UnsealTriad(m, PlanTriad)
	}

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Adapt (via contradiction) should unseal Plan but NEVER unseal Reason")
	}
	if m.TriadSealed(PlanTriad) {
		t.Error("Plan should be unsealed after Adapt contradiction")
	}
}

package reactivity

import "testing"

func TestTriad_ReasonSeals(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))

	if c.TriadSealed(ReasonTriad) {
		t.Error("Reason should not seal after Intent only")
	}

	c.Add(mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if !c.TriadSealed(ReasonTriad) {
		t.Error("Reason should seal after Intent + Assessment")
	}
	if c.CurrentTriad() != PlanTriad {
		t.Errorf("should advance to Plan triad, got %s", c.CurrentTriad())
	}
}

func TestTriad_PlanSeals(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if c.TriadSealed(PlanTriad) {
		t.Error("Plan should not seal before any plan atoms")
	}

	c.Add(mkAtom("option", PlanAtom, "plan.option.cook", Fresh))

	if !c.TriadSealed(PlanTriad) {
		t.Error("Plan should seal after plan atom")
	}
	if c.CurrentTriad() != ActTriad {
		t.Errorf("should advance to Act triad, got %s", c.CurrentTriad())
	}
}

func TestTriad_ActSeals(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(mkAtom("learning", RetrospectionAtom, "retrospection.learning.done", Fresh))

	if !c.TriadSealed(ActTriad) {
		t.Error("Act should seal after Execution + Retrospection")
	}
	if !c.AllTriadsSealed() {
		t.Error("all triads should be sealed after full chain")
	}
}

func TestTriad_TriadOfMapping(t *testing.T) {
	tests := []struct {
		atomType AtomType
		triad    Triad
	}{
		{IntentAtom, ReasonTriad},
		{AssessmentAtom, ReasonTriad},
		{PlanAtom, PlanTriad},
		{ExecutionAtom, ActTriad},
		{RetrospectionAtom, ActTriad},
	}
	for _, tt := range tests {
		got := TriadOf(tt.atomType)
		if got != tt.triad {
			t.Errorf("TriadOf(%s) = %s, want %s", tt.atomType, got, tt.triad)
		}
	}
}

func TestTriad_AssessmentAcceptedInPlanTriad(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if c.CurrentTriad() != PlanTriad {
		t.Fatalf("expected Plan triad, got %s", c.CurrentTriad())
	}

	result, _ := c.Add(mkAtom("late-finding", AssessmentAtom, "assessment.state.time-late", Fresh))
	if result == Incompatible {
		t.Error("Assessment should be accepted even in Plan triad (promiscuous)")
	}
}

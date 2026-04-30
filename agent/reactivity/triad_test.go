package reactivity

import "testing"

func TestTriad_ReasonSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))

	if m.TriadSealed(ReasonTriad) {
		t.Error("Reason should not seal after Intent only")
	}

	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Reason should seal after Intent + Assessment")
	}
	if m.CurrentTriad() != PlanTriad {
		t.Errorf("should advance to Plan triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_PlanSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if m.TriadSealed(PlanTriad) {
		t.Error("Plan should not seal before any plan atoms")
	}

	c.Add(m, mkAtom("option", PlanAtom, "plan.option.cook", Fresh))

	if !m.TriadSealed(PlanTriad) {
		t.Error("Plan should seal after plan atom")
	}
	if m.CurrentTriad() != ActTriad {
		t.Errorf("should advance to Act triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_ActSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))

	if !m.TriadSealed(ActTriad) {
		t.Error("Act should seal after Execution")
	}

	c.Add(m, mkAtom("learning", RetrospectionAtom, "retrospection.learning.done", Fresh))

	if !m.TriadSealed(RetrospectTriad) {
		t.Error("Retrospect should seal after Retrospection atom")
	}
	if !m.AllTriadsSealed() {
		t.Error("all 4 triads should be sealed after full chain")
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
		{RetrospectionAtom, RetrospectTriad},
	}
	for _, tt := range tests {
		got := TriadOf(tt.atomType)
		if got != tt.triad {
			t.Errorf("TriadOf(%s) = %s, want %s", tt.atomType, got, tt.triad)
		}
	}
}

func TestTriad_AssessmentAcceptedInPlanTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if m.CurrentTriad() != PlanTriad {
		t.Fatalf("expected Plan triad, got %s", m.CurrentTriad())
	}

	result, _ := c.Add(m, mkAtom("late-finding", AssessmentAtom, "assessment.state.time-late", Fresh))
	if result == Incompatible {
		t.Error("Assessment should be accepted even in Plan triad (promiscuous)")
	}
}

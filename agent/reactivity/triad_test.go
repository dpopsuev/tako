package reactivity

import "testing"

func TestTriad_ReasonSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if m.TriadSealed(ReasonTriad) {
		t.Error("Reason should not seal before synthesis")
	}

	c.Add(m, mkAtom("understood", UnderstandingAtom, "understanding.synth.eat", Fresh))

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Reason should seal after thesis + antithesis + synthesis")
	}
	if m.CurrentTriad() != PlanTriad {
		t.Errorf("should advance to Plan triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_PlanSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	c.Add(m, mkAtom("option", PlanAtom, "plan.option.cook", Fresh))
	c.Add(m, mkAtom("danger", RiskAtom, "risk.eval.cook", Fresh))
	c.Add(m, mkAtom("approach", StrategyAtom, "strategy.synth.cook", Fresh))

	if !m.TriadSealed(PlanTriad) {
		t.Error("Plan should seal after thesis + antithesis + synthesis")
	}
	if m.CurrentTriad() != ActTriad {
		t.Errorf("should advance to Act triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_ActSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")

	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("saw", ObservationAtom, "observation.eval.swept", Fresh))
	c.Add(m, mkAtom("adjusted", AdaptationAtom, "adaptation.synth.swept", Fresh))

	if !m.TriadSealed(ActTriad) {
		t.Error("Act should seal after thesis + antithesis + synthesis")
	}

	c.Add(m, mkAtom("learning", RetrospectionAtom, "retrospection.reflect.done", Fresh))

	if !m.TriadSealed(RetrospectTriad) {
		t.Error("Retrospect should seal after Reflection")
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
		{UnderstandingAtom, ReasonTriad},
		{PlanAtom, PlanTriad},
		{RiskAtom, PlanTriad},
		{StrategyAtom, PlanTriad},
		{ExecutionAtom, ActTriad},
		{ObservationAtom, ActTriad},
		{AdaptationAtom, ActTriad},
		{RetrospectionAtom, RetrospectTriad},
	}
	for _, tt := range tests {
		got := TriadOf(tt.atomType)
		if got != tt.triad {
			t.Errorf("TriadOf(%s) = %s, want %s", tt.atomType, got, tt.triad)
		}
	}
}

func TestTriad_PositionOfMapping(t *testing.T) {
	tests := []struct {
		atomType AtomType
		pos      DialecticPosition
	}{
		{IntentAtom, ThesisPosition},
		{AssessmentAtom, AntithesisPosition},
		{UnderstandingAtom, SynthesisPosition},
		{PlanAtom, ThesisPosition},
		{RiskAtom, AntithesisPosition},
		{StrategyAtom, SynthesisPosition},
		{ExecutionAtom, ThesisPosition},
		{ObservationAtom, AntithesisPosition},
		{AdaptationAtom, SynthesisPosition},
	}
	for _, tt := range tests {
		got := PositionOf(tt.atomType)
		if got != tt.pos {
			t.Errorf("PositionOf(%s) = %d, want %d", tt.atomType, got, tt.pos)
		}
	}
}

func TestTriad_AssessmentAcceptedInPlanTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	if m.CurrentTriad() != PlanTriad {
		t.Fatalf("expected Plan triad, got %s", m.CurrentTriad())
	}

	result, _ := c.Add(m, mkAtom("late-finding", AssessmentAtom, "assessment.state.time-late", Fresh))
	if result == Incompatible {
		t.Error("Assessment should be accepted even in Plan triad (promiscuous)")
	}
}

package reactivity

import "testing"

func TestTriad_ReasonSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if m.TriadSealed(ThinkTriad) {
		t.Error("Reason should not seal before synthesis")
	}

	c.Add(m, mkAtom("understood", KnowledgeAtom, "understanding.synth.eat", Fresh))

	if !m.TriadSealed(ThinkTriad) {
		t.Error("Reason should seal after thesis + antithesis + synthesis")
	}
	if m.CurrentTriad() != ComposeTriad {
		t.Errorf("should advance to Plan triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_PlanSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	c.Add(m, mkAtom("option", ExpansionAtom, "plan.option.cook", Fresh))
	c.Add(m, mkAtom("danger", ReductionAtom, "risk.eval.cook", Fresh))
	c.Add(m, mkAtom("approach", SelectionAtom, "strategy.synth.cook", Fresh))

	if !m.TriadSealed(ComposeTriad) {
		t.Error("Plan should seal after thesis + antithesis + synthesis")
	}
	if m.CurrentTriad() != ImplementTriad {
		t.Errorf("should advance to Act triad, got %s", m.CurrentTriad())
	}
}

func TestTriad_ActSeals(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")

	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("saw", AcclimationAtom, "observation.eval.swept", Fresh))
	c.Add(m, mkAtom("adjusted", RefinementAtom, "adaptation.synth.swept", Fresh))

	if !m.TriadSealed(ImplementTriad) {
		t.Error("Act should seal after thesis + antithesis + synthesis")
	}

	c.Add(m, mkAtom("learning", RetrospectionAtom, "retrospection.reflect.done", Fresh))

	if !m.TriadSealed(ReflectTriad) {
		t.Error("Retrospect should seal after Reflection")
	}
	if !m.AllTriadsSealed() {
		t.Error("all 4 triads should be sealed after full chain")
	}
}

func TestAtomType_TriadField(t *testing.T) {
	tests := []struct {
		atomType AtomType
		triad    Triad
	}{
		{IntentAtom, ThinkTriad},
		{AssessmentAtom, ThinkTriad},
		{KnowledgeAtom, ThinkTriad},
		{ExpansionAtom, ComposeTriad},
		{ReductionAtom, ComposeTriad},
		{SelectionAtom, ComposeTriad},
		{ExecutionAtom, ImplementTriad},
		{AcclimationAtom, ImplementTriad},
		{RefinementAtom, ImplementTriad},
		{RetrospectionAtom, ReflectTriad},
	}
	for _, tt := range tests {
		if tt.atomType.Triad != tt.triad {
			t.Errorf("%s.Triad = %s, want %s", tt.atomType, tt.atomType.Triad, tt.triad)
		}
	}
}

func TestAtomType_PositionField(t *testing.T) {
	tests := []struct {
		atomType AtomType
		pos      DialecticPosition
	}{
		{IntentAtom, ThesisPosition},
		{AssessmentAtom, AntithesisPosition},
		{KnowledgeAtom, SynthesisPosition},
		{ExpansionAtom, ThesisPosition},
		{ReductionAtom, AntithesisPosition},
		{SelectionAtom, SynthesisPosition},
		{ExecutionAtom, ThesisPosition},
		{AcclimationAtom, AntithesisPosition},
		{RefinementAtom, SynthesisPosition},
	}
	for _, tt := range tests {
		if tt.atomType.Position != tt.pos {
			t.Errorf("%s.Position = %d, want %d", tt.atomType, tt.atomType.Position, tt.pos)
		}
	}
}

func TestTriad_AssessmentAcceptedInComposeTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	if m.CurrentTriad() != ComposeTriad {
		t.Fatalf("expected Plan triad, got %s", m.CurrentTriad())
	}

	result, _ := c.Add(m, mkAtom("late-finding", AssessmentAtom, "assessment.state.time-late", Fresh))
	if result == Incompatible {
		t.Error("Assessment should be accepted even in Plan triad (promiscuous)")
	}
}

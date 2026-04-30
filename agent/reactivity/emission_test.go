package reactivity

import "testing"

func TestMolecule_Emit(t *testing.T) {
	m := NewMolecule("emit-test")

	m.Emit(Emission{Kind: "instrument", Target: "grep", Payload: []byte(`{"pattern":"error"}`)})
	m.Emit(Emission{Kind: "instrument", Target: "read", Payload: []byte(`{"path":"main.go"}`)})

	emissions := m.Emissions()
	if len(emissions) != 2 {
		t.Fatalf("expected 2 emissions, got %d", len(emissions))
	}
	if emissions[0].Target != "grep" {
		t.Errorf("expected grep, got %s", emissions[0].Target)
	}
}

func TestMolecule_DrainEmissions(t *testing.T) {
	m := NewMolecule("drain-test")

	m.Emit(Emission{Kind: "wish", Target: "monolog", Payload: []byte("done")})
	m.Emit(Emission{Kind: "instrument", Target: "write", Payload: []byte("data")})

	drained := m.DrainEmissions()
	if len(drained) != 2 {
		t.Fatalf("expected 2 drained, got %d", len(drained))
	}

	if len(m.Emissions()) != 0 {
		t.Error("emissions should be empty after drain")
	}
}

func TestMolecule_Emit_EmptyByDefault(t *testing.T) {
	m := NewMolecule("empty-test")
	if len(m.Emissions()) != 0 {
		t.Error("new molecule should have zero emissions")
	}
}

func TestReactor_React_CanEmit(t *testing.T) {
	emitting := &emittingReactor{}
	c := NewReactor(WithTriad(ActTriad, emitting))

	m := NewMolecule("reactor-emit")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.test", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.test", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.test", Fresh))
	c.Add(m, mkAtom("exec", ExecutionAtom, "execution.result.test", Fresh))

	emissions := m.DrainEmissions()
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission from act reactor, got %d", len(emissions))
	}
	if emissions[0].Kind != "instrument" {
		t.Errorf("expected instrument emission, got %s", emissions[0].Kind)
	}
}

type emittingReactor struct{}

func (emittingReactor) React(m *Molecule, atom Atom) (AssertResult, Fortune) {
	m.Emit(Emission{Kind: "instrument", Target: "test-tool", Payload: atom.Content})
	if m.mass[ExecutionAtom] > 0 {
		m.SealTriad(ActTriad)
		m.SetPhase(RetrospectionAtom)
		return Pass, Fortune{}
	}
	return Insufficient, Fortune{Result: Insufficient, Message: "need execution atoms", Phase: ExecutionAtom}
}

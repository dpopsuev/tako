package reactivity

import "testing"

func TestMolecule_Emit(t *testing.T) {
	m := NewMolecule("emit-test")

	m.Emit(Emission{Kind: "organ", Target: "grep", Payload: []byte(`{"pattern":"error"}`)})
	m.Emit(Emission{Kind: "organ", Target: "read", Payload: []byte(`{"path":"main.go"}`)})

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
	m.Emit(Emission{Kind: "organ", Target: "write", Payload: []byte("data")})

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
	c := NewReactor(WithTriad(ImplementTriad, emitting))

	m := NewMolecule("reactor-emit")
	addReasonAtoms(c, m, "test")
	addFormationAtoms(c, m, "test")
	c.Add(m, mkAtom("exec", ExecutionAtom, "execution.result.test", Fresh))

	emissions := m.DrainEmissions()
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission from act reactor, got %d", len(emissions))
	}
	if emissions[0].Kind != "organ" {
		t.Errorf("expected instrument emission, got %s", emissions[0].Kind)
	}
}

type emittingReactor struct{}

func (emittingReactor) React(m *Molecule, atom Atom) (YieldKind, Yield) {
	m.Emit(Emission{Kind: "organ", Target: "test-tool", Payload: atom.Content})
	if m.mass[RefinementAtom] > 0 {
		m.SealTriad(ImplementTriad)
		m.SetPhase(RetrospectionAtom)
		return Pass, Yield{}
	}
	return Insufficient, Yield{Result: Insufficient, Message: "need adaptation atom", Phase: ExecutionAtom}
}

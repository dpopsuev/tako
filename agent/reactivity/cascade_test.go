package reactivity

import "testing"

func TestCascade_UnsealPlanDoesNotUnsealReason(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")

	if !m.TriadSealed(ThinkTriad) {
		t.Fatal("Reason should be sealed")
	}
	if !m.TriadSealed(ComposeTriad) {
		t.Fatal("Plan should be sealed")
	}

	c.UnsealTriad(m, ComposeTriad)

	if !m.TriadSealed(ThinkTriad) {
		t.Error("Reason should STAY sealed when Plan unseals")
	}
	if m.TriadSealed(ComposeTriad) {
		t.Error("Plan should be unsealed")
	}
	if m.TriadSealed(ActionTriad) {
		t.Error("Act should be unsealed (cascade down from Plan)")
	}
}

func TestCascade_UnsealReasonCascadesAll(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addFullChain(c, m, "clean")

	if !m.AllTriadsSealed() {
		t.Fatal("all triads should be sealed after full chain")
	}

	c.UnsealTriad(m, ThinkTriad)

	if m.TriadSealed(ThinkTriad) {
		t.Error("Reason should be unsealed")
	}
	if m.TriadSealed(ComposeTriad) {
		t.Error("Plan should be unsealed (cascade from Reason)")
	}
	if m.TriadSealed(ActionTriad) {
		t.Error("Act should be unsealed (cascade from Reason)")
	}
}

func TestCascade_UnsealActOnlyAffectsAct(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addFullChain(c, m, "clean")

	c.UnsealTriad(m, ActionTriad)

	if !m.TriadSealed(ThinkTriad) {
		t.Error("Reason should stay sealed")
	}
	if !m.TriadSealed(ComposeTriad) {
		t.Error("Plan should stay sealed")
	}
	if m.TriadSealed(ActionTriad) {
		t.Error("Act should be unsealed")
	}
}

func TestCascade_RetrospectOrthogonal(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addFullChain(c, m, "clean")

	if !m.AllTriadsSealed() {
		t.Fatal("all 4 triads should be sealed")
	}

	c.UnsealTriad(m, ThinkTriad)

	if m.TriadSealed(ThinkTriad) || m.TriadSealed(ComposeTriad) || m.TriadSealed(ActionTriad) {
		t.Error("Reason/Plan/Act should be unsealed by cascade")
	}
	if !m.TriadSealed(ReflectTriad) {
		t.Error("Retrospect should remain sealed — orthogonal to cascade")
	}
}

func TestCascade_AdaptNeverUnsealsReason(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "floor")
	addFormationAtoms(c, m, "floor")
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.floor", Fresh))

	dirty := mkAtom("dirty-again", AcclimationAtom, "observation.state.floor", Fresh)
	contradicts, _ := c.Contradict(m, dirty)
	if contradicts {
		c.UnsealTriad(m, ComposeTriad)
	}

	if !m.TriadSealed(ThinkTriad) {
		t.Error("Adapt (via contradiction) should unseal Plan but NEVER unseal Reason")
	}
	if m.TriadSealed(ComposeTriad) {
		t.Error("Plan should be unsealed after Adapt contradiction")
	}
}

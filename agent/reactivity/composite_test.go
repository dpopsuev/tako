package reactivity

import "testing"

func TestComposite_ReactInterface(t *testing.T) {
	var r Reactor = NewReactor()
	m := NewMolecule("interface-test")

	result, _ := r.React(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	if result != Pass {
		t.Fatalf("expected Pass, got %s", result)
	}
}

func TestComposite_WithTriad_Ablation(t *testing.T) {
	c := NewReactor(WithTriad(ComposeTriad, Damper{}))
	m := NewMolecule("ablation")

	addReasonAtoms(c, m, "eat")

	if !m.TriadSealed(ThinkTriad) {
		t.Error("reason triad should seal normally")
	}

	c.Add(m, mkAtom("task", ExpansionAtom, "plan.task.cook", Fresh))
	c.Add(m, mkAtom("risk", ReductionAtom, "risk.eval.burn", Fresh))
	c.Add(m, mkAtom("strat", SelectionAtom, "strategy.synth.cook", Fresh))

	if m.TriadSealed(ComposeTriad) {
		t.Error("plan triad should NOT seal — Damper reactor does not advance")
	}
}

func TestComposite_NestedReactors(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("nested")

	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	addActionAtoms(c, m, "clean")
	c.Add(m, mkAtom("retro", RetrospectionAtom, "retrospection.learning.done", Fresh))

	if !m.AllTriadsSealed() {
		t.Error("all 4 triads should seal through nested reactors")
	}
}

func TestComposite_Damper_PassesThrough(t *testing.T) {
	noop := Damper{}
	m := NewMolecule("noop-test")
	atom := mkAtom("a1", IntentAtom, "intent.test.noop", Fresh)

	result, _ := noop.React(m, atom)
	if result != Pass {
		t.Errorf("Damper should always pass, got %s", result)
	}
	if m.Mass(IntentAtom) != 0 {
		t.Error("Damper should not insert atoms — Core.React does InsertAtom before delegating")
	}
}

func TestComposite_Add_BackwardCompat(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("compat")

	result, _ := c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	if result != Pass {
		t.Errorf("Add (backward compat) should work, got %s", result)
	}
}

func TestComposite_SealDelegatesToMolecule(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("seal-test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))

	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))
	if !m.Sealed() {
		t.Error("Seal should mark molecule as sealed")
	}
}

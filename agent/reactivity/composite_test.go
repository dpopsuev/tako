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
	c := NewReactor(WithTriad(PlanTriad, Damper{}))
	m := NewMolecule("ablation")

	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	if !m.TriadSealed(ReasonTriad) {
		t.Error("Reason should seal normally")
	}

	c.Add(m, mkAtom("task", PlanAtom, "plan.task.cook", Fresh))

	if m.TriadSealed(PlanTriad) {
		t.Error("Plan triad should NOT seal — Damper reactor doesn't advance")
	}
}

func TestComposite_NestedReactors(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("nested")

	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
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
	if m.Mass(IntentAtom) != 1 {
		t.Error("Damper should still insert the atom")
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

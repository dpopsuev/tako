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

func TestComposite_WithTriad_GimpedNodes(t *testing.T) {
	gimped := NewTriadReactor(ComposeTriad,
		[3]AtomType{ExpansionAtom, ReductionAtom, SelectionAtom},
		ExecutionAtom,
	)
	c := NewReactor(WithTriad(ComposeTriad, gimped))
	m := NewMolecule("ablation")

	addReasonAtoms(c, m, "eat")

	if !m.TriadSealed(ThinkTriad) {
		t.Error("think triad should seal normally")
	}

	if !gimped.Thesis().Gimped() {
		t.Error("thesis node should be gimped (no directives)")
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

func TestComposite_GimpedNode_PassesThrough(t *testing.T) {
	node := GimpedNode(IntentAtom)
	m := NewMolecule("gimped-test")
	atom := mkAtom("a1", IntentAtom, "intent.test.gimped", Fresh)

	result, _ := node.React(m, atom)
	if result != Pass {
		t.Errorf("gimped node should always pass, got %s", result)
	}
	if !node.Gimped() {
		t.Error("node without directives should be gimped")
	}
}

func TestComposite_NodeWithDirective_NotGimped(t *testing.T) {
	node := NewNode(IntentAtom, "Focus on explicit request")
	if node.Gimped() {
		t.Error("node with directive[0] should not be gimped")
	}
	if len(node.Directives()) != 1 {
		t.Errorf("expected 1 directive, got %d", len(node.Directives()))
	}
}

func TestComposite_NodeRemoveDirective_Gimps(t *testing.T) {
	node := NewNode(IntentAtom, "default")
	node.AddDirective("extra")

	if len(node.Directives()) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(node.Directives()))
	}

	node.RemoveDirective(0)
	node.RemoveDirective(0)

	if !node.Gimped() {
		t.Error("removing all directives should gimp the node")
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

func TestCore_WithDirective_DelegatesToNode(t *testing.T) {
	c := NewReactor(
		WithDirective(IntentAtom, "Focus on explicit request"),
		WithDirective(IntentAtom, "Ask for clarification"),
	)

	node := c.Node(IntentAtom)
	if node == nil {
		t.Fatal("Node(IntentAtom) should not be nil")
	}
	if len(node.Directives()) != 2 {
		t.Errorf("expected 2 directives on Intent node, got %d", len(node.Directives()))
	}
}

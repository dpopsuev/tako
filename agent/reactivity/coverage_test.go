package reactivity

import "testing"

func TestAllAtomTypes(t *testing.T) {
	all := AllAtomTypes()
	if len(all) != 10 {
		t.Fatalf("expected 10 atom types, got %d", len(all))
	}
	if all[0] != IntentAtom {
		t.Errorf("first atom should be IntentAtom, got %s", all[0])
	}
	if all[9] != RetrospectionAtom {
		t.Errorf("last atom should be RetrospectionAtom, got %s", all[9])
	}
}

func TestAtomType_String_Unknown(t *testing.T) {
	unknown := AtomType{Triad(99), DialecticPosition(99)}
	s := unknown.String()
	if s == "" {
		t.Error("unknown atom type should still produce a string")
	}
}

func TestYieldKind_String_All(t *testing.T) {
	cases := []struct {
		kind YieldKind
		want string
	}{
		{Pass, "PASS"},
		{Insufficient, "INSUFFICIENT"},
		{Incompatible, "INCOMPATIBLE"},
		{Unresolvable, "UNRESOLVABLE"},
		{Contradiction, "CONTRADICTION"},
		{YieldKind(99), "UNKNOWN"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("YieldKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestDialecticPosition_String(t *testing.T) {
	cases := []struct {
		pos  DialecticPosition
		want string
	}{
		{ThesisPosition, "thesis"},
		{AntithesisPosition, "antithesis"},
		{SynthesisPosition, "synthesis"},
		{DialecticPosition(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.pos.String(); got != tc.want {
			t.Errorf("DialecticPosition(%d).String() = %q, want %q", tc.pos, got, tc.want)
		}
	}
}

func TestUnsealKind_String(t *testing.T) {
	cases := []struct {
		kind UnsealKind
		want string
	}{
		{Recognize, "recognize"},
		{Rethink, "rethink"},
		{Recompose, "recompose"},
		{Reaction, "reaction"},
		{UnsealKind(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("UnsealKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestTriadNodes_Generic(t *testing.T) {
	nodes := TriadNodes(ThinkTriad)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].Triad != ThinkTriad || nodes[0].Position != ThesisPosition {
		t.Errorf("first node should be Think/Thesis, got %s", nodes[0])
	}
	if nodes[2].Position != SynthesisPosition {
		t.Errorf("third node should be synthesis, got %s", nodes[2])
	}
}

func TestTriadReactor_Accessors(t *testing.T) {
	tr := NewTriadReactor(ComposeTriad, ExecutionAtom)
	if tr.Antithesis() == nil {
		t.Error("Antithesis() should return a node")
	}
	if tr.Synthesis() == nil {
		t.Error("Synthesis() should return a node")
	}
}

func TestCore_WithReflect(t *testing.T) {
	custom := &customSink{fired: false}
	c := NewReactor(WithReflect(custom))
	m := NewMolecule("sink-test")
	addFullChain(c, m, "test")
	if !custom.fired {
		t.Error("custom sink should have fired")
	}
}

type customSink struct{ fired bool }

func (s *customSink) React(m *Molecule, _ Atom) (YieldKind, Yield) {
	s.fired = true
	if m.mass[RetrospectionAtom] > 0 {
		m.SealTriad(ReflectTriad)
		return Pass, Yield{}
	}
	return Insufficient, Yield{}
}

func TestCore_AddDirective_And_Directives(t *testing.T) {
	c := NewReactor()
	c.AddDirective(IntentAtom, "test directive")
	dirs := c.Directives(IntentAtom)
	if len(dirs) != 1 {
		t.Errorf("expected 1 directive, got %d", len(dirs))
	}
}

func TestMolecule_Atom_Accessor(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom})
	a, ok := m.Atom("a1")
	if !ok || a.ID != "a1" {
		t.Error("Atom accessor should return inserted atom")
	}
	_, ok = m.Atom("missing")
	if ok {
		t.Error("Atom accessor should return false for missing")
	}
}

func TestMolecule_IsSealed(t *testing.T) {
	m := NewMolecule("test")
	if m.IsSealed() {
		t.Error("new molecule should not be sealed")
	}
}

func TestMolecule_UnsealCount(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addFullChain(c, m, "test")
	c.UnsealTriad(m, ThinkTriad)
	if m.UnsealCount() == 0 {
		t.Error("unseal count should be > 0 after unseal")
	}
}

func TestMolecule_Contradict_EmptyTaxonomy(t *testing.T) {
	m := NewMolecule("test")
	atom := Atom{ID: "a1", Type: IntentAtom, Taxonomy: ""}
	contradicts, _ := m.Contradict(atom)
	if contradicts {
		t.Error("empty taxonomy should not contradict")
	}
}

func TestNode_Label(t *testing.T) {
	n := NewNode(IntentAtom, "intent", "directive")
	if n.Label() != "intent" {
		t.Errorf("expected label 'intent', got %q", n.Label())
	}
}

func TestNode_Phase(t *testing.T) {
	n := GimpedNode(ExpansionAtom)
	if n.Phase() != ExpansionAtom {
		t.Errorf("expected ExpansionAtom phase, got %s", n.Phase())
	}
}

func TestNode_RemoveDirective_OutOfRange(t *testing.T) {
	n := NewNode(IntentAtom, "intent", "d")
	n.RemoveDirective(-1)
	n.RemoveDirective(99)
	if len(n.Directives()) != 1 {
		t.Error("out of range remove should not change directives")
	}
}

func TestReflection_Insufficient(t *testing.T) {
	r := Reflection{}
	m := NewMolecule("test")
	result, _ := r.React(m, Atom{})
	if result != Insufficient {
		t.Errorf("empty molecule should yield Insufficient from Reflection, got %s", result)
	}
}

func TestAtomDomain_NoDot(t *testing.T) {
	m := NewMolecule("test")
	atom := Atom{ID: "a1", Type: IntentAtom, Taxonomy: "nodot"}
	contradicts, _ := m.Contradict(atom)
	if contradicts {
		t.Error("taxonomy without dot should not contradict")
	}
}

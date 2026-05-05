package reactivity

import (
	"testing"
)

func TestLinearNavigator(t *testing.T) {
	m := NewMolecule("nav-1")
	cases := []struct {
		sealed Triad
		want   AtomType
	}{
		{ThinkTriad, ExpansionAtom},
		{ComposeTriad, ExecutionAtom},
		{ImplementTriad, RetrospectionAtom},
	}
	for _, tc := range cases {
		got := LinearNavigator(m, tc.sealed)
		if got != tc.want {
			t.Errorf("Linear(%s) = %s, want %s", tc.sealed, got, tc.want)
		}
	}
}

func TestTreeNavigator_ShortcutsOnLowDistance(t *testing.T) {
	cat := Catalyst{
		Need:    "simple task",
		Desired: map[string]any{"done": true},
	}
	m := NewMoleculeWithCatalyst("nav-2", cat)
	m.ReportSensor("done", true) // distance = 0, already met

	got := TreeNavigator(m, ThinkTriad)
	// With distance 0, should skip expansion/reduction → go straight to Selection
	if got != SelectionAtom {
		t.Errorf("TreeNavigator should go to Selection when distance=0, got %s", got)
	}
}

func TestTreeNavigator_ShortcutsToExecutionWithRecollection(t *testing.T) {
	cat := Catalyst{
		Need:    "known task",
		Desired: map[string]any{"done": true},
	}
	m := NewMoleculeWithCatalyst("nav-2b", cat)
	m.ReportSensor("done", true) // distance = 0
	// Add recollected atoms to trigger recollection shortcut
	m.InsertAtom(Atom{ID: "r1", Type: KnowledgeAtom, Source: Recollected, Content: []byte("known")})
	m.InsertAtom(Atom{ID: "r2", Type: ExpansionAtom, Source: Recollected, Content: []byte("plan")})
	m.InsertAtom(Atom{ID: "f1", Type: IntentAtom, Source: Fresh, Content: []byte("need")})

	got := TreeNavigator(m, ThinkTriad)
	// High recollection ratio + low distance → shortcut to Execution
	if got != ExecutionAtom {
		t.Errorf("TreeNavigator should shortcut to Execution with recollection+low distance, got %s", got)
	}
}

func TestTreeNavigator_FullPathOnHighDistance(t *testing.T) {
	cat := Catalyst{
		Need:    "complex task",
		Desired: map[string]any{"a": true, "b": true, "c": true, "d": true},
	}
	m := NewMoleculeWithCatalyst("nav-3", cat)
	// distance = 1.0 (nothing met)

	got := TreeNavigator(m, ThinkTriad)
	// With distance 1.0, should go to Expansion (full compose path)
	if got != ExpansionAtom {
		t.Errorf("TreeNavigator should go to Expansion on high distance, got %s", got)
	}
}

func TestTreeNavigator_SkipToExecutionAfterCompose(t *testing.T) {
	cat := Catalyst{
		Need:    "task with plan",
		Desired: map[string]any{"done": true},
	}
	m := NewMoleculeWithCatalyst("nav-4", cat)
	// Compose is done, should go to Execution
	got := TreeNavigator(m, ComposeTriad)
	if got != ExecutionAtom {
		t.Errorf("after Compose, should go to Execution, got %s", got)
	}
}

func TestTreeNavigator_ReflectAlwaysGoesToRetrospection(t *testing.T) {
	m := NewMolecule("nav-5")
	got := TreeNavigator(m, ImplementTriad)
	if got != RetrospectionAtom {
		t.Errorf("after Implement, should go to Retrospection, got %s", got)
	}
}

func TestNavigatorOnTriadReactor_ViaCore(t *testing.T) {
	// Use Core to verify Navigator is called — Core walks through triads properly
	coreLinear := NewReactor() // default = LinearNavigator
	m := NewMolecule("nav-6")

	// Add atoms to fill Think triad
	coreLinear.Add(m, Atom{ID: "a1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("test")})
	coreLinear.Add(m, Atom{ID: "a2", Type: AssessmentAtom, Taxonomy: "assessment.x", Content: []byte("x")})
	coreLinear.Add(m, Atom{ID: "a3", Type: KnowledgeAtom, Taxonomy: "knowledge.x", Content: []byte("x")})

	if !m.TriadSealed(ThinkTriad) {
		t.Fatal("Think triad should be sealed")
	}
	if m.Phase() != ExpansionAtom {
		t.Errorf("LinearNavigator: after Think sealed, phase should be Expansion, got %s", m.Phase())
	}

	// Now test with TreeNavigator
	coreTree := NewReactor(WithNavigator(TreeNavigator))
	m2 := NewMoleculeWithCatalyst("nav-7", Catalyst{
		Need:    "complex",
		Desired: map[string]any{"a": true, "b": true, "c": true, "d": true},
	})

	coreTree.Add(m2, Atom{ID: "b1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("test")})
	coreTree.Add(m2, Atom{ID: "b2", Type: AssessmentAtom, Taxonomy: "assessment.x", Content: []byte("x")})
	coreTree.Add(m2, Atom{ID: "b3", Type: KnowledgeAtom, Taxonomy: "knowledge.x", Content: []byte("x")})

	if !m2.TriadSealed(ThinkTriad) {
		t.Fatal("Think triad should be sealed")
	}
	// distance=1.0 (nothing met) → TreeNavigator should go to Expansion (full path)
	if m2.Phase() != ExpansionAtom {
		t.Errorf("TreeNavigator with high distance: after Think sealed, phase should be Expansion, got %s", m2.Phase())
	}
}

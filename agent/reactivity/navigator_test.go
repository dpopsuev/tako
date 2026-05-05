package reactivity

import (
	"testing"
)

func TestLinearNavigator(t *testing.T) {
	m := NewMolecule("nav-1")
	cases := []struct {
		current AtomType
		want    AtomType
	}{
		{IntentAtom, AssessmentAtom},
		{AssessmentAtom, KnowledgeAtom},
		{KnowledgeAtom, ExpansionAtom},
		{ExpansionAtom, ReductionAtom},
		{ReductionAtom, SelectionAtom},
		{SelectionAtom, ExecutionAtom},
		{ExecutionAtom, AcclimationAtom},
		{AcclimationAtom, RefinementAtom},
		{RefinementAtom, RetrospectionAtom},
	}
	for _, tc := range cases {
		got := LinearNavigator(m, tc.current)
		if got != tc.want {
			t.Errorf("Linear(%s) = %s, want %s", tc.current, got, tc.want)
		}
	}
}

func TestTreeNavigator_ShortcutsOnLowDistance(t *testing.T) {
	cat := Catalyst{
		Need:    "simple task",
		Desired: map[string]any{"done": true},
	}
	m := NewMoleculeWithCatalyst("nav-2", cat)
	m.ReportSensor("done", true) // distance = 0

	got := TreeNavigator(m, IntentAtom)
	// After Intent with distance=0 → shortcut to Selection (skip Assessment+Knowledge+Expansion+Reduction)
	if got != SelectionAtom {
		t.Errorf("TreeNavigator after Intent with distance=0 should go to Selection, got %s", got)
	}
}

func TestTreeNavigator_ShortcutsToExecutionWithRecollection(t *testing.T) {
	cat := Catalyst{
		Need:    "known task",
		Desired: map[string]any{"done": true},
	}
	m := NewMoleculeWithCatalyst("nav-2b", cat)
	m.ReportSensor("done", true) // distance = 0
	m.InsertAtom(Atom{ID: "r1", Type: KnowledgeAtom, Source: Recollected, Content: []byte("known")})
	m.InsertAtom(Atom{ID: "r2", Type: ExpansionAtom, Source: Recollected, Content: []byte("plan")})
	m.InsertAtom(Atom{ID: "f1", Type: IntentAtom, Source: Fresh, Content: []byte("need")})

	got := TreeNavigator(m, IntentAtom)
	// Recollection >30% + distance<0.3 → shortcut straight to Execution
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

	got := TreeNavigator(m, IntentAtom)
	// distance=1.0 → need assessment, no shortcut
	if got != AssessmentAtom {
		t.Errorf("TreeNavigator after Intent with high distance should go to Assessment, got %s", got)
	}
}

func TestTreeNavigator_SkipAfterAssessment(t *testing.T) {
	cat := Catalyst{
		Need:    "medium task",
		Desired: map[string]any{"done": true, "checked": true, "verified": true},
	}
	m := NewMoleculeWithCatalyst("nav-4", cat)
	m.ReportSensor("done", true)     // 1 of 3 met
	m.ReportSensor("checked", true)  // 2 of 3 met → distance = 0.33

	got := TreeNavigator(m, AssessmentAtom)
	// distance=0.33 < 0.5 → skip to Selection
	if got != SelectionAtom {
		t.Errorf("TreeNavigator after Assessment with distance=0.33 should skip to Selection, got %s", got)
	}
}

func TestTreeNavigator_SelectionAlwaysToExecution(t *testing.T) {
	m := NewMolecule("nav-5")
	got := TreeNavigator(m, SelectionAtom)
	if got != ExecutionAtom {
		t.Errorf("after Selection should always go to Execution, got %s", got)
	}
}

func TestTreeNavigator_ExecutionAlwaysToAcclimation(t *testing.T) {
	m := NewMolecule("nav-6")
	got := TreeNavigator(m, ExecutionAtom)
	if got != AcclimationAtom {
		t.Errorf("after Execution should go to Acclimation, got %s", got)
	}
}

func TestTreeNavigator_AcclimationSkipsRefinementOnZeroDistance(t *testing.T) {
	cat := Catalyst{Desired: map[string]any{"done": true}}
	m := NewMoleculeWithCatalyst("nav-7", cat)
	m.ReportSensor("done", true) // distance = 0

	got := TreeNavigator(m, AcclimationAtom)
	if got != RetrospectionAtom {
		t.Errorf("after Acclimation with distance=0 should skip to Retrospection, got %s", got)
	}
}

func TestNavigatorOnTriadReactor_ViaCore(t *testing.T) {
	coreLinear := NewReactor()
	m := NewMolecule("nav-core-1")

	coreLinear.Add(m, Atom{ID: "a1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("test")})
	coreLinear.Add(m, Atom{ID: "a2", Type: AssessmentAtom, Taxonomy: "assessment.x", Content: []byte("x")})
	coreLinear.Add(m, Atom{ID: "a3", Type: KnowledgeAtom, Taxonomy: "knowledge.x", Content: []byte("x")})

	if !m.TriadSealed(ThinkTriad) {
		t.Fatal("Think triad should be sealed")
	}
	if m.Phase() != ExpansionAtom {
		t.Errorf("LinearNavigator: after Think sealed, phase should be Expansion, got %s", m.Phase())
	}
}

func TestTreeNavigatorOnCore_ShortcutsAfterIntent(t *testing.T) {
	coreTree := NewReactor(WithNavigator(TreeNavigator))
	// Desired has 4 dimensions, 3 already met → distance = 0.25 (< 0.3)
	cat := Catalyst{Need: "mostly known", Desired: map[string]any{"a": true, "b": true, "c": true, "d": true}}
	m := NewMoleculeWithCatalyst("nav-core-2", cat)
	m.ReportSensor("a", true)
	m.ReportSensor("b", true)
	m.ReportSensor("c", true)
	// d not met → distance = 0.25, NOT sealed
	// Add recollected atoms for high recollection ratio
	m.InsertAtom(Atom{ID: "r1", Type: KnowledgeAtom, Source: Recollected, Content: []byte("known")})
	m.InsertAtom(Atom{ID: "r2", Type: ExpansionAtom, Source: Recollected, Content: []byte("plan")})
	m.InsertAtom(Atom{ID: "r3", Type: SelectionAtom, Source: Recollected, Content: []byte("commit")})

	coreTree.Add(m, Atom{ID: "a1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("test")})

	// distance=0.25 + recollection=3/4=0.75 → should shortcut past Think to Execution
	if m.Phase().Triad == ThinkTriad {
		t.Errorf("TreeNavigator should have skipped Think triad, still at %s (distance=%.2f, recollection=%d/%d)",
			m.Phase(), m.Distance(), m.SourceMass(Recollected), m.TotalMass())
	}
}

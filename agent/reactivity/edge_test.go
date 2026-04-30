package reactivity

import "testing"

func TestMolecule_AddEdge(t *testing.T) {
	m := NewMolecule("edge-test")

	m.AddEdge("a1", "a2", Thesis)
	m.AddEdge("a1", "a3", Antithesis)
	m.AddEdge("a2", "a3", Synthesis)

	edges := m.Edges()
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}

	if edges[0].Kind != Thesis {
		t.Errorf("expected thesis, got %s", edges[0].Kind)
	}
	if edges[1].Kind != Antithesis {
		t.Errorf("expected antithesis, got %s", edges[1].Kind)
	}
	if edges[2].Kind != Synthesis {
		t.Errorf("expected synthesis, got %s", edges[2].Kind)
	}
}

func TestMolecule_EdgesFrom_BackwardCompat(t *testing.T) {
	m := NewMolecule("compat-test")

	m.AddEdge("a1", "a2", Reference)
	m.AddEdge("a1", "a3", Reference)
	m.AddEdge("a2", "a4", Reference)

	targets := m.EdgesFrom("a1")
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets from a1, got %d", len(targets))
	}
	if targets[0] != "a2" || targets[1] != "a3" {
		t.Errorf("expected [a2, a3], got %v", targets)
	}

	targets = m.EdgesFrom("a2")
	if len(targets) != 1 || targets[0] != "a4" {
		t.Errorf("expected [a4] from a2, got %v", targets)
	}
}

func TestMolecule_TypedEdgesFrom(t *testing.T) {
	m := NewMolecule("typed-test")

	m.AddEdge("a1", "a2", Thesis)
	m.AddEdge("a1", "a3", Antithesis)
	m.AddEdge("a1", "a4", Reference)

	typed := m.TypedEdgesFrom("a1")
	if len(typed) != 3 {
		t.Fatalf("expected 3 typed edges, got %d", len(typed))
	}

	kinds := map[EdgeKind]int{}
	for _, e := range typed {
		kinds[e.Kind]++
	}
	if kinds[Thesis] != 1 || kinds[Antithesis] != 1 || kinds[Reference] != 1 {
		t.Errorf("unexpected kind distribution: %v", kinds)
	}
}

func TestMolecule_EdgesByKind(t *testing.T) {
	m := NewMolecule("filter-test")

	m.AddEdge("a1", "a2", Thesis)
	m.AddEdge("a2", "a3", Antithesis)
	m.AddEdge("a3", "a4", Synthesis)
	m.AddEdge("a1", "a5", Reference)

	theses := m.EdgesByKind(Thesis)
	if len(theses) != 1 || theses[0].From != "a1" {
		t.Errorf("expected 1 thesis from a1, got %v", theses)
	}

	syntheses := m.EdgesByKind(Synthesis)
	if len(syntheses) != 1 || syntheses[0].From != "a3" {
		t.Errorf("expected 1 synthesis from a3, got %v", syntheses)
	}
}

func TestEdgeKind_String(t *testing.T) {
	cases := []struct {
		kind EdgeKind
		want string
	}{
		{Reference, "reference"},
		{Thesis, "thesis"},
		{Antithesis, "antithesis"},
		{Synthesis, "synthesis"},
		{EdgeKind(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("EdgeKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestReactor_Add_CreatesReferenceEdges(t *testing.T) {
	r := NewReactor()
	m := NewMolecule("ref-edge-test")

	r.Add(m, Atom{ID: "intent-1", Type: IntentAtom, Taxonomy: "intent.goal.test"})
	r.Add(m, Atom{ID: "assess-1", Type: AssessmentAtom, Taxonomy: "assessment.eval.test", Targets: []string{"intent-1"}})

	edges := m.TypedEdgesFrom("assess-1")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge from assess-1, got %d", len(edges))
	}
	if edges[0].To != "intent-1" || edges[0].Kind != Reference {
		t.Errorf("expected reference edge to intent-1, got %v", edges[0])
	}
}

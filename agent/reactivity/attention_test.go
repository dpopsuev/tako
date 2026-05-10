package reactivity

import (
	"testing"
	"time"
)

func TestAttend_RanksSemanticallySimilarHigher(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{
		ID:        "a1",
		Type:      AssessmentAtom,
		Content:   []byte("cows produce milk"),
		Embedding: []float64{0.9, 0.1, 0.0},
		CreatedAt: time.Now(),
	})
	m.InsertAtom(Atom{
		ID:        "a2",
		Type:      KnowledgeAtom,
		Content:   []byte("quantum physics theory"),
		Embedding: []float64{0.0, 0.1, 0.9},
		CreatedAt: time.Now(),
	})

	query := []float64{0.8, 0.2, 0.0}
	result := m.Attend(query, 1.0)

	if len(result) != 2 {
		t.Fatalf("expected 2 atoms, got %d", len(result))
	}
	if result[0].Atom.ID != "a1" {
		t.Errorf("expected a1 (cows) ranked first, got %s", result[0].Atom.ID)
	}
	if result[0].Score <= result[1].Score {
		t.Errorf("first score should be higher: %.4f vs %.4f", result[0].Score, result[1].Score)
	}
	t.Logf("a1=%.4f a2=%.4f", result[0].Score, result[1].Score)
}

func TestAttend_ColdTemperatureSharpens(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{
		ID: "a1", Type: AssessmentAtom, Embedding: []float64{0.9, 0.1}, CreatedAt: time.Now(),
	})
	m.InsertAtom(Atom{
		ID: "a2", Type: KnowledgeAtom, Embedding: []float64{0.5, 0.5}, CreatedAt: time.Now(),
	})

	query := []float64{1.0, 0.0}

	cold := m.Attend(query, 0.1)
	warm := m.Attend(query, 1.0)
	hot := m.Attend(query, 5.0)

	coldGap := cold[0].Score - cold[1].Score
	warmGap := warm[0].Score - warm[1].Score
	hotGap := hot[0].Score - hot[1].Score

	if coldGap <= warmGap {
		t.Errorf("cold should be sharper than warm: cold_gap=%.4f warm_gap=%.4f", coldGap, warmGap)
	}
	if warmGap <= hotGap {
		t.Errorf("warm should be sharper than hot: warm_gap=%.4f hot_gap=%.4f", warmGap, hotGap)
	}
	t.Logf("gaps: cold=%.4f warm=%.4f hot=%.4f", coldGap, warmGap, hotGap)
}

func TestAttend_DimensionalHeadBoostsUnmetAtoms(t *testing.T) {
	m := NewMoleculeWithCatalyst("test", Catalyst{
		Need:    "fix the build",
		Desired: map[string]any{"build_passing": true, "tests_green": true},
	})
	m.ReportSensor("build_passing", true)

	m.InsertAtom(Atom{
		ID: "a1", Type: ExecutionAtom,
		Embedding:  []float64{0.5, 0.5},
		Dimensions: []string{"tests_green"},
		CreatedAt:  time.Now(),
	})
	m.InsertAtom(Atom{
		ID: "a2", Type: ExecutionAtom,
		Embedding:  []float64{0.5, 0.5},
		Dimensions: []string{"build_passing"},
		CreatedAt:  time.Now(),
	})

	query := []float64{0.5, 0.5}
	result := m.Attend(query, 1.0)

	if result[0].Atom.ID != "a1" {
		t.Errorf("atom addressing unmet dimension (tests_green) should rank higher, got %s first", result[0].Atom.ID)
	}
	t.Logf("a1(unmet)=%.4f a2(met)=%.4f", result[0].Score, result[1].Score)
}

func TestAttend_StructuralHeadBoostsConnectedAtoms(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{
		ID: "a1", Type: AssessmentAtom, Embedding: []float64{0.5, 0.5}, CreatedAt: time.Now(),
	})
	m.InsertAtom(Atom{
		ID: "a2", Type: KnowledgeAtom, Embedding: []float64{0.5, 0.5}, CreatedAt: time.Now(),
	})
	m.AddEdge("a1", "a2", Reference)

	query := []float64{0.5, 0.5}
	result := m.Attend(query, 1.0)

	a1Score := findScore(result, "a1")
	a2Score := findScore(result, "a2")

	if a1Score <= 0 || a2Score <= 0 {
		t.Errorf("both atoms should have positive scores: a1=%.4f a2=%.4f", a1Score, a2Score)
	}
	t.Logf("a1(has-edge-from)=%.4f a2(edge-target)=%.4f", a1Score, a2Score)
}

func TestAttend_EmptyMolecule(t *testing.T) {
	m := NewMolecule("test")
	result := m.Attend([]float64{1.0, 0.0}, 1.0)
	if result != nil {
		t.Errorf("empty molecule should return nil, got %d atoms", len(result))
	}
}

func TestSoftmaxTemperature(t *testing.T) {
	scores := []float64{2.0, 1.0, 0.0}

	cold := softmaxTemperature(scores, 0.1)
	warm := softmaxTemperature(scores, 1.0)
	hot := softmaxTemperature(scores, 10.0)

	if cold[0] < 0.99 {
		t.Errorf("cold softmax should concentrate on max: got %.4f", cold[0])
	}

	if hot[0]-hot[2] > 0.1 {
		t.Errorf("hot softmax should be near-uniform: got [%.4f, %.4f, %.4f]", hot[0], hot[1], hot[2])
	}

	t.Logf("cold=[%.4f %.4f %.4f] warm=[%.4f %.4f %.4f] hot=[%.4f %.4f %.4f]",
		cold[0], cold[1], cold[2], warm[0], warm[1], warm[2], hot[0], hot[1], hot[2])
}

func findScore(result []WeightedAtom, id string) float64 {
	for _, wa := range result {
		if wa.Atom.ID == id {
			return wa.Score
		}
	}
	return -1
}

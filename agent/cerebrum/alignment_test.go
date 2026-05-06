package cerebrum

import (
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestCheckStructural_EarlyTurn(t *testing.T) {
	m := reactivity.NewMolecule("test")
	result := checkStructural(m)
	if !result.Aligned || result.Score != 1 {
		t.Fatalf("early turns should be aligned, got score=%v aligned=%v", result.Score, result.Aligned)
	}
}

func TestCheckStructural_Regressing(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": "done", "b": "done"},
	})
	m.Tick()
	m.Tick()
	m.Tick()

	result := checkStructural(m)
	if result.Score != 0.5 {
		t.Fatalf("stuck distance (delta=0) should score 0.5, got %v", result.Score)
	}
}

func TestCheckDimensional_ExactOverlap(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"hungry": "fed"},
	})
	atom := reactivity.Atom{
		Dimensions: []string{"hungry"},
	}

	result := checkDimensional(atom, m)
	if result.Score != 1 {
		t.Fatalf("exact overlap should score 1.0, got %v", result.Score)
	}
	if !result.Aligned {
		t.Fatal("should be aligned")
	}
}

func TestCheckDimensional_NoOverlap(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"hungry": "fed"},
	})
	atom := reactivity.Atom{
		Dimensions: []string{"furniture"},
	}

	result := checkDimensional(atom, m)
	if result.Score != 0 {
		t.Fatalf("no overlap should score 0, got %v", result.Score)
	}
	if result.Aligned {
		t.Fatal("should detect drift")
	}
	if len(result.DriftFlags) == 0 || result.DriftFlags[0] != "no_unmet_overlap" {
		t.Fatalf("should flag no_unmet_overlap, got %v", result.DriftFlags)
	}
}

func TestCheckDimensional_WastedWork(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"hungry": "fed", "tired": "rested"},
	})
	m.ReportSensor("hungry", "fed")

	atom := reactivity.Atom{
		Dimensions: []string{"hungry"},
	}

	result := checkDimensional(atom, m)
	hasWasted := false
	for _, f := range result.DriftFlags {
		if f == "wasted_dimensions" {
			hasWasted = true
		}
	}
	if !hasWasted {
		t.Fatalf("should flag wasted_dimensions, got %v", result.DriftFlags)
	}
}

func TestCheckDimensional_NoDimensions(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": "done"},
	})
	atom := reactivity.Atom{}

	result := checkDimensional(atom, m)
	if !result.Aligned {
		t.Fatal("no dimensions = permissive, should be aligned")
	}
	if len(result.DriftFlags) == 0 || result.DriftFlags[0] != "no_dimensions_claimed" {
		t.Fatalf("should flag no_dimensions_claimed, got %v", result.DriftFlags)
	}
}

func TestTieredAlignment(t *testing.T) {
	checker := TieredAlignment{}
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "cook eggs",
		Desired: map[string]any{"eggs": "cooked"},
	})
	atom := reactivity.Atom{
		Dimensions: []string{"eggs"},
	}
	result := checker.Check(atom, m)
	if !result.Aligned {
		t.Fatal("should be aligned")
	}
}

func TestSynthesisDiff_NoPrevious(t *testing.T) {
	m := reactivity.NewMolecule("test")
	diff := m.SynthesisDiff(reactivity.ThinkTriad)
	if diff != 1.0 {
		t.Fatalf("no synthesis should return 1.0, got %v", diff)
	}
}

func TestSynthesisDiff_Identical(t *testing.T) {
	m := reactivity.NewMolecule("test")
	m.InsertAtom(reactivity.Atom{
		ID:      "s1",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("the answer is 42"),
		CreatedAt: time.Now(),
	})
	m.InsertAtom(reactivity.Atom{
		ID:      "s2",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("the answer is 42"),
		CreatedAt: time.Now(),
	})
	diff := m.SynthesisDiff(reactivity.ThinkTriad)
	if diff != 0 {
		t.Fatalf("identical synthesis should return 0, got %v", diff)
	}
}

func TestSynthesisDiff_Different(t *testing.T) {
	m := reactivity.NewMolecule("test")
	m.InsertAtom(reactivity.Atom{
		ID:      "s1",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("aaaa"),
		CreatedAt: time.Now(),
	})
	m.InsertAtom(reactivity.Atom{
		ID:      "s2",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("bbbb"),
		CreatedAt: time.Now(),
	})
	diff := m.SynthesisDiff(reactivity.ThinkTriad)
	if diff != 1.0 {
		t.Fatalf("completely different should return 1.0, got %v", diff)
	}
}

func TestConvergenceAssert_ConvergedButStuck(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": "done"},
	})
	m.InsertAtom(reactivity.Atom{
		ID:      "s1",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("answer"),
		CreatedAt: time.Now(),
	})
	m.InsertAtom(reactivity.Atom{
		ID:      "s2",
		Type:    reactivity.KnowledgeAtom,
		Content: []byte("answer"),
		CreatedAt: time.Now(),
	})
	m.Tick()
	m.Tick()
	m.Tick()

	assert := ConvergenceAssert{
		Inner: reactivity.DefaultAssert,
	}
	crit := assert.Evaluate(m)
	if crit != reactivity.Subcritical {
		t.Fatalf("converged + stuck should be subcritical, got %v", crit)
	}
}

func TestConvergenceAssert_DelegatesBase(t *testing.T) {
	m := reactivity.NewMolecule("test")
	assert := ConvergenceAssert{
		Inner: reactivity.DefaultAssert,
	}
	crit := assert.Evaluate(m)
	if crit != reactivity.Critical {
		t.Fatalf("should delegate to inner, got %v", crit)
	}
}

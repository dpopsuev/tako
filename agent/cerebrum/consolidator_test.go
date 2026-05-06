package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
)

func newTestMolecule() *reactivity.Molecule {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test task",
		Desired: map[string]any{"done": true},
	})
	m.InsertAtom(reactivity.Atom{
		ID:       "a1",
		Type:     reactivity.IntentAtom,
		Taxonomy: "intent.test",
		Content:  []byte("understand the task"),
	})
	m.InsertAtom(reactivity.Atom{
		ID:       "a2",
		Type:     reactivity.SelectionAtom,
		Taxonomy: "selection.test",
		Content:  []byte("plan: do the thing"),
	})
	return m
}

func TestConsolidator_CreateKnowledge(t *testing.T) {
	mesh := memory.NewInMemoryMesh()
	c := NewMeshConsolidator(mesh)

	m := newTestMolecule()
	if err := c.Consolidate(m, []byte("test")); err != nil {
		t.Fatal(err)
	}

	nodes := mesh.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Tier != memory.Knowledge {
		t.Errorf("first consolidation should be Knowledge tier, got %d", nodes[0].Tier)
	}
}

func TestConsolidator_PromoteToUnderstanding(t *testing.T) {
	mesh := memory.NewInMemoryMesh()
	c := NewMeshConsolidator(mesh)
	c.PromotionCount = 2

	m := newTestMolecule()
	c.Consolidate(m, []byte("test"))
	c.Consolidate(m, []byte("test"))
	c.Consolidate(m, []byte("test"))

	nodes := mesh.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node (promoted in place), got %d", len(nodes))
	}
	if nodes[0].Tier != memory.Understanding {
		t.Errorf("should be promoted to Understanding, got %d", nodes[0].Tier)
	}
}

func TestConsolidator_PromoteToWisdom(t *testing.T) {
	mesh := memory.NewInMemoryMesh()
	c := NewMeshConsolidator(mesh)
	c.PromotionCount = 1
	c.WisdomCount = 2

	m := newTestMolecule()

	c.Consolidate(m, []byte("test"))
	c.Consolidate(m, []byte("test"))
	c.Consolidate(m, []byte("test"))
	c.Consolidate(m, []byte("test"))

	nodes := mesh.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Tier != memory.Wisdom {
		t.Errorf("should be promoted to Wisdom, got %d", nodes[0].Tier)
	}
}

func TestConsolidator_DifferentPatterns(t *testing.T) {
	mesh := memory.NewInMemoryMesh()
	c := NewMeshConsolidator(mesh)

	m1 := reactivity.NewMoleculeWithCatalyst("m1", reactivity.Catalyst{
		Need: "task1", Desired: map[string]any{"a": true},
	})
	m2 := reactivity.NewMoleculeWithCatalyst("m2", reactivity.Catalyst{
		Need: "task2", Desired: map[string]any{"b": true},
	})

	c.Consolidate(m1, []byte("task1"))
	c.Consolidate(m2, []byte("task2"))

	nodes := mesh.Nodes()
	if len(nodes) != 2 {
		t.Errorf("different patterns should create separate nodes, got %d", len(nodes))
	}
}

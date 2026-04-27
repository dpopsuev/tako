package memory

import (
	"errors"
	"testing"
	"time"
)

func TestStubMeshAddAndRetrieve(t *testing.T) {
	mesh := NewStubMesh()
	node := KnowledgeNode{ID: "n1", Content: "fact", Tier: Knowledge, CreatedAt: time.Now()}
	if err := mesh.AddNode(node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	got, err := mesh.Node("n1")
	if err != nil {
		t.Fatalf("Node failed: %v", err)
	}
	if got.Content != "fact" {
		t.Errorf("expected content 'fact', got %q", got.Content)
	}
}

func TestStubMeshNotFound(t *testing.T) {
	mesh := NewStubMesh()
	_, err := mesh.Node("missing")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestStubMeshEdgesAndNeighbors(t *testing.T) {
	mesh := NewStubMesh()
	now := time.Now()
	_ = mesh.AddNode(KnowledgeNode{ID: "a", Content: "a", CreatedAt: now})
	_ = mesh.AddNode(KnowledgeNode{ID: "b", Content: "b", CreatedAt: now})
	_ = mesh.AddEdge(Edge{From: "a", To: "b", Relation: "related", CreatedAt: now})

	neighbors, err := mesh.Neighbors("a")
	if err != nil {
		t.Fatalf("Neighbors failed: %v", err)
	}
	if len(neighbors) != 1 || neighbors[0].ID != "b" {
		t.Errorf("expected neighbor [b], got %v", neighbors)
	}
}

func TestStubMeshNodes(t *testing.T) {
	mesh := NewStubMesh()
	_ = mesh.AddNode(KnowledgeNode{ID: "x", Content: "x", CreatedAt: time.Now()})
	_ = mesh.AddNode(KnowledgeNode{ID: "y", Content: "y", CreatedAt: time.Now()})
	nodes := mesh.Nodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

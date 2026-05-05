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

func TestInMemoryMesh_BFSWalk(t *testing.T) {
	mesh := NewInMemoryMesh()
	now := time.Now()
	mesh.AddNode(KnowledgeNode{ID: "a", Content: "root", CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "b", Content: "child1", CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "c", Content: "child2", CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "d", Content: "grandchild", CreatedAt: now})
	mesh.AddEdge(Edge{From: "a", To: "b", CreatedAt: now})
	mesh.AddEdge(Edge{From: "a", To: "c", CreatedAt: now})
	mesh.AddEdge(Edge{From: "b", To: "d", CreatedAt: now})

	var visited []string
	var depths []int
	mesh.Walk("a", func(n KnowledgeNode, depth int) bool {
		visited = append(visited, n.ID)
		depths = append(depths, depth)
		return true
	})

	if len(visited) != 4 {
		t.Fatalf("expected 4 nodes visited, got %d: %v", len(visited), visited)
	}
	if visited[0] != "a" {
		t.Errorf("first visited should be root 'a', got %s", visited[0])
	}
	if depths[0] != 0 || depths[len(depths)-1] != 2 {
		t.Errorf("depths should range 0-2, got %v", depths)
	}
}

func TestInMemoryMesh_WalkStopEarly(t *testing.T) {
	mesh := NewInMemoryMesh()
	now := time.Now()
	mesh.AddNode(KnowledgeNode{ID: "a", Content: "root", CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "b", Content: "child", CreatedAt: now})
	mesh.AddEdge(Edge{From: "a", To: "b", CreatedAt: now})

	var count int
	mesh.Walk("a", func(n KnowledgeNode, depth int) bool {
		count++
		return false
	})

	if count != 1 {
		t.Errorf("walk should stop after first node, got %d", count)
	}
}

func TestInMemoryMesh_NodesByTier(t *testing.T) {
	mesh := NewInMemoryMesh()
	now := time.Now()
	mesh.AddNode(KnowledgeNode{ID: "k1", Tier: Knowledge, CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "k2", Tier: Knowledge, CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "u1", Tier: Understanding, CreatedAt: now})
	mesh.AddNode(KnowledgeNode{ID: "w1", Tier: Wisdom, CreatedAt: now})

	knowledge := mesh.NodesByTier(Knowledge)
	if len(knowledge) != 2 {
		t.Errorf("expected 2 Knowledge nodes, got %d", len(knowledge))
	}

	wisdom := mesh.NodesByTier(Wisdom)
	if len(wisdom) != 1 {
		t.Errorf("expected 1 Wisdom node, got %d", len(wisdom))
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

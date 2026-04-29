package memory

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dpopsuev/tako/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestDoltMeshAddAndRetrieveNode(t *testing.T) {
	db := openTestDB(t)
	mesh := NewDoltMesh(db.DB)

	err := mesh.AddNode(KnowledgeNode{
		ID:        "n1",
		Content:   "test content",
		Tier:      Knowledge,
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	node, err := mesh.Node("n1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if node.Content != "test content" {
		t.Errorf("expected 'test content', got %q", node.Content)
	}
}

func TestDoltMeshAddEdgeAndNeighbors(t *testing.T) {
	db := openTestDB(t)
	mesh := NewDoltMesh(db.DB)

	mesh.AddNode(KnowledgeNode{ID: "a", Content: "alpha", CreatedAt: time.Now()})
	mesh.AddNode(KnowledgeNode{ID: "b", Content: "beta", CreatedAt: time.Now()})
	mesh.AddEdge(Edge{From: "a", To: "b", Relation: "relates", Weight: 1.0, CreatedAt: time.Now()})

	neighbors, err := mesh.Neighbors("a")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(neighbors) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(neighbors))
	}
	if neighbors[0].ID != "b" {
		t.Errorf("expected neighbor 'b', got %q", neighbors[0].ID)
	}
}

func TestDoltMeshWalk(t *testing.T) {
	db := openTestDB(t)
	mesh := NewDoltMesh(db.DB)

	mesh.AddNode(KnowledgeNode{ID: "root", Content: "root", CreatedAt: time.Now()})
	mesh.AddNode(KnowledgeNode{ID: "child1", Content: "child1", CreatedAt: time.Now()})
	mesh.AddNode(KnowledgeNode{ID: "child2", Content: "child2", CreatedAt: time.Now()})
	mesh.AddEdge(Edge{From: "root", To: "child1", Relation: "has", Weight: 1.0, CreatedAt: time.Now()})
	mesh.AddEdge(Edge{From: "root", To: "child2", Relation: "has", Weight: 1.0, CreatedAt: time.Now()})

	var visited []string
	err := mesh.Walk("root", func(node KnowledgeNode, depth int) bool {
		visited = append(visited, node.ID)
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(visited) != 3 {
		t.Errorf("expected 3 visited nodes, got %d: %v", len(visited), visited)
	}
}

func TestDoltMeshNodeNotFound(t *testing.T) {
	db := openTestDB(t)
	mesh := NewDoltMesh(db.DB)

	_, err := mesh.Node("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestDoltMeshNodesAndEdges(t *testing.T) {
	db := openTestDB(t)
	mesh := NewDoltMesh(db.DB)

	mesh.AddNode(KnowledgeNode{ID: "x", Content: "x", CreatedAt: time.Now()})
	mesh.AddNode(KnowledgeNode{ID: "y", Content: "y", CreatedAt: time.Now()})
	mesh.AddEdge(Edge{From: "x", To: "y", Relation: "links", Weight: 0.5, CreatedAt: time.Now()})

	nodes := mesh.Nodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	edges := mesh.Edges()
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}

func TestDoltMeshPersistsAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "restartdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mesh := NewDoltMesh(db.DB)
	mesh.AddNode(KnowledgeNode{ID: "persist", Content: "survives", CreatedAt: time.Now()})
	db.Close()

	db2, err := store.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if err := db2.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	mesh2 := NewDoltMesh(db2.DB)
	node, err := mesh2.Node("persist")
	if err != nil {
		t.Fatalf("Node after restart: %v", err)
	}
	if node.Content != "survives" {
		t.Errorf("expected 'survives', got %q", node.Content)
	}
}

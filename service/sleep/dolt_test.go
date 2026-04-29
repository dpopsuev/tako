package sleep

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/store"
)

func TestDoltDrainSweepWritesToMesh(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	monolog := &discourse.StubMonolog{}
	monolog.Write(discourse.Letter{
		From:      "worker-0",
		Subject:   "executed oculus",
		Body:      "completed station triage",
		CreatedAt: time.Now(),
	})

	mesh := memory.NewDoltMesh(db.DB)
	drain := NewDoltDrain(monolog)

	if err := drain.Sweep(mesh); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	nodes := mesh.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after drain, got %d", len(nodes))
	}
	if nodes[0].Content != "completed station triage" {
		t.Errorf("expected drained content, got %q", nodes[0].Content)
	}
}

func TestDoltDrainSweepMultipleLetters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	monolog := &discourse.StubMonolog{}
	monolog.Write(discourse.Letter{From: "w0", Subject: "task-1", Body: "done 1", CreatedAt: time.Now()})
	monolog.Write(discourse.Letter{From: "w0", Subject: "task-2", Body: "done 2", CreatedAt: time.Now()})
	monolog.Write(discourse.Letter{From: "w0", Subject: "task-3", Body: "done 3", CreatedAt: time.Now()})

	mesh := memory.NewDoltMesh(db.DB)
	drain := NewDoltDrain(monolog)

	if err := drain.Sweep(mesh); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	nodes := mesh.Nodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes after drain, got %d", len(nodes))
	}
}

func TestDoltDrainPersistsAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	monolog := &discourse.StubMonolog{}
	monolog.Write(discourse.Letter{From: "w0", Subject: "persist-test", Body: "survives restart", CreatedAt: time.Now()})

	mesh := memory.NewDoltMesh(db.DB)
	drain := NewDoltDrain(monolog)
	drain.Sweep(mesh)
	db.Close()

	db2, err := store.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if err := db2.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	mesh2 := memory.NewDoltMesh(db2.DB)
	nodes := mesh2.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after restart, got %d", len(nodes))
	}
	if nodes[0].Content != "survives restart" {
		t.Errorf("expected 'survives restart', got %q", nodes[0].Content)
	}
}

package cerebrum

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dpopsuev/tako/store"
)

func openTestDoltDB(t *testing.T) *store.DB {
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

func TestDoltPipeStore_AddAndMatch(t *testing.T) {
	db := openTestDoltDB(t)
	s := NewDoltPipeStore(db.DB)

	embedding := []float64{1.0, 0.0, 0.0, 0.0}
	pipe := Pipe{
		Name:        "test-pipe",
		Description: "test description",
		Embedding:   embedding,
		Steps: []PipeStep{
			{ID: "s1", Call: "file.read", Confidence: 0.8},
			{ID: "s2", Call: "edit", Confidence: 0.7, DependsOn: []string{"s1"}},
		},
	}

	if err := s.Add(pipe); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}

	matched, sim := s.Match(embedding)
	if matched == nil {
		t.Fatal("Match returned nil")
	}
	if sim < 0.999 {
		t.Errorf("similarity = %f, want ~1.0", sim)
	}
	if matched.Name != "test-pipe" {
		t.Errorf("name = %q, want %q", matched.Name, "test-pipe")
	}
	if len(matched.Steps) != 2 {
		t.Errorf("steps = %d, want 2", len(matched.Steps))
	}
	if matched.Steps[0].Call != "file.read" {
		t.Errorf("step[0].Call = %q, want %q", matched.Steps[0].Call, "file.read")
	}
}

func TestDoltPipeStore_AddDuplicateErrors(t *testing.T) {
	db := openTestDoltDB(t)
	s := NewDoltPipeStore(db.DB)

	pipe := Pipe{Name: "dup", Embedding: []float64{1, 0}}
	if err := s.Add(pipe); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := s.Add(pipe); err == nil {
		t.Error("second Add should error on duplicate")
	}
}

func TestDoltPipeStore_MergeIncreasesConfidence(t *testing.T) {
	db := openTestDoltDB(t)
	s := NewDoltPipeStore(db.DB)

	embedding := []float64{0.9, 0.1, 0.0, 0.0}
	pipe := Pipe{
		Name:      "merge-target",
		Embedding: embedding,
		Steps:     []PipeStep{{ID: "s1", Call: "grep", Confidence: 0.6}},
	}
	s.Add(pipe)

	merged := s.Merge(embedding, []PipeStep{{ID: "s1", Call: "grep", Confidence: 0.6}})
	if !merged {
		t.Fatal("Merge should return true for similar embedding")
	}

	matched, _ := s.Match(embedding)
	if matched == nil {
		t.Fatal("Match after merge returned nil")
	}
	if matched.Steps[0].Confidence <= 0.6 {
		t.Errorf("confidence should increase after merge, got %f", matched.Steps[0].Confidence)
	}
}

func TestDoltPipeStore_PruneRemovesLowScoring(t *testing.T) {
	db := openTestDoltDB(t)
	s := NewDoltPipeStore(db.DB)

	s.Add(Pipe{Name: "good", Embedding: []float64{1, 0}, Replays: 10, Usage: 10})
	s.Add(Pipe{Name: "bad", Embedding: []float64{0, 1}, Replays: 0, Usage: 100})

	pruned := s.Prune(0.3)
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}
	if s.Len() != 1 {
		t.Errorf("Len after prune = %d, want 1", s.Len())
	}
}

func TestDoltPipeStore_SurvivesReconnect(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "persist-test")
	db1, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	db1.Migrate()

	s1 := NewDoltPipeStore(db1.DB)
	s1.Add(Pipe{
		Name:       "persistent",
		Embedding:  []float64{1, 0, 0},
		Steps:      []PipeStep{{ID: "s1", Call: "bash", Confidence: 0.9}},
		LastPlayed: time.Now(),
	})
	s1.Save()
	db1.Close()

	db2, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer db2.Close()
	db2.Migrate()

	s2 := NewDoltPipeStore(db2.DB)
	if s2.Len() != 1 {
		t.Fatalf("Len after reconnect = %d, want 1", s2.Len())
	}

	matched, sim := s2.Match([]float64{1, 0, 0})
	if matched == nil {
		t.Fatal("Match after reconnect returned nil")
	}
	if sim < 0.999 {
		t.Errorf("similarity after reconnect = %f", sim)
	}
	if matched.Name != "persistent" {
		t.Errorf("name = %q, want %q", matched.Name, "persistent")
	}
	if len(matched.Steps) != 1 {
		t.Errorf("steps = %d, want 1", len(matched.Steps))
	}
}

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected database directory to exist")
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM envelopes"); err != nil {
		t.Fatalf("query envelopes: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 envelopes, got %d", count)
	}
}

func TestInsertAndQueryEnvelope(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO envelopes (id, shelf_name, origin, payload, hash, state) VALUES (?, ?, ?, ?, ?, ?)",
		"env-1", "intake", "test", []byte("hello"), "abc123", "UNCLAIMED",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var payload []byte
	if err := db.Get(&payload, "SELECT payload FROM envelopes WHERE id = ?", "env-1"); err != nil {
		t.Fatalf("query: %v", err)
	}
	if string(payload) != "hello" {
		t.Errorf("expected 'hello', got %q", payload)
	}
}

func TestInsertAndQueryKnowledgeNode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO knowledge_nodes (id, content, tier) VALUES (?, ?, ?)",
		"node-1", "test content", 0,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var content string
	if err := db.Get(&content, "SELECT content FROM knowledge_nodes WHERE id = ?", "node-1"); err != nil {
		t.Fatalf("query: %v", err)
	}
	if content != "test content" {
		t.Errorf("expected 'test content', got %q", content)
	}
}

func TestErgographRecordChain(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO ergograph_records (identity, action, timestamp, sequence, hash, prev_hash) VALUES (?, ?, NOW(), ?, ?, ?)",
		"worker-0", "shelf.push", 0, "hash0", "",
	)
	if err != nil {
		t.Fatalf("insert record 0: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO ergograph_records (identity, action, timestamp, sequence, hash, prev_hash) VALUES (?, ?, NOW(), ?, ?, ?)",
		"worker-0", "shelf.pull", 1, "hash1", "hash0",
	)
	if err != nil {
		t.Fatalf("insert record 1: %v", err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM ergograph_records"); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 records, got %d", count)
	}
}

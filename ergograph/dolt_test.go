package ergograph

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

func TestDoltPoolAppend(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	err := pool.Append(Record{
		Identity:  "worker-0",
		Action:    "shelf.push",
		Timestamp: time.Now(),
		Labels:    map[string]string{"station": "intake"},
		Payload:   []byte("test"),
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	if pool.Len() != 1 {
		t.Errorf("expected 1 record, got %d", pool.Len())
	}
}

func TestDoltPoolHashChain(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	for i := range 5 {
		err := pool.Append(Record{
			Identity:  "worker-0",
			Action:    "action",
			Timestamp: time.Now(),
			Labels:    map[string]string{"i": string(rune('0' + i))},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	if err := pool.VerifyChain(); err != nil {
		t.Errorf("chain verification failed: %v", err)
	}

	if pool.Len() != 5 {
		t.Errorf("expected 5 records, got %d", pool.Len())
	}
}

func TestDoltPoolRecordsOrdered(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	_ = pool.Append(Record{Identity: "w", Action: "first", Timestamp: time.Now()})
	_ = pool.Append(Record{Identity: "w", Action: "second", Timestamp: time.Now()})
	_ = pool.Append(Record{Identity: "w", Action: "third", Timestamp: time.Now()})

	recs := pool.Records()
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0].Action != "first" {
		t.Errorf("expected first, got %q", recs[0].Action)
	}
	if recs[2].Action != "third" {
		t.Errorf("expected third, got %q", recs[2].Action)
	}
}

func TestDoltPoolChainLinksCorrectly(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	_ = pool.Append(Record{Identity: "w", Action: "a", Timestamp: time.Now()})
	_ = pool.Append(Record{Identity: "w", Action: "b", Timestamp: time.Now()})

	recs := pool.Records()
	if recs[0].PrevHash != "" {
		t.Errorf("first record should have empty PrevHash, got %q", recs[0].PrevHash)
	}
	if recs[1].PrevHash != recs[0].Hash {
		t.Errorf("second record PrevHash should match first Hash")
	}
	if recs[0].Hash == "" || recs[1].Hash == "" {
		t.Error("hashes should not be empty")
	}
}

func TestDoltPoolInspectorScores(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	_ = pool.Append(Record{Identity: "w", Action: "a", Timestamp: time.Now()})

	inspector := StubInspector{}
	oae, err := inspector.Score(pool)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if oae.Score() != 1.0 {
		t.Errorf("expected OAE 1.0, got %f", oae.Score())
	}
	if err := inspector.Verify(pool); err != nil {
		t.Errorf("verify: %v", err)
	}
}

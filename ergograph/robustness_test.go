package ergograph

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dpopsuev/tako/store"
)

func TestDoltPoolTamperDetection(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	_ = pool.Append(Record{Identity: "w", Action: "a", Timestamp: time.Now()})
	_ = pool.Append(Record{Identity: "w", Action: "b", Timestamp: time.Now()})
	_ = pool.Append(Record{Identity: "w", Action: "c", Timestamp: time.Now()})

	_, err := db.Exec("UPDATE ergograph_records SET hash = 'tampered' WHERE sequence = 1")
	if err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := pool.VerifyChain(); err == nil {
		t.Error("expected chain verification to fail after tampering")
	}
}

func TestDoltPoolDoltRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "restartdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool := NewDoltPool(db.DB)
	_ = pool.Append(Record{Identity: "w", Action: "before-restart", Timestamp: time.Now()})
	db.Close()

	db2, err := store.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if err := db2.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	var count int
	if err := db2.Get(&count, "SELECT COUNT(*) FROM ergograph_records"); err != nil {
		t.Fatalf("query after restart: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record after restart, got %d", count)
	}
}

func TestDoltPoolLoad1000(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	start := time.Now()
	for i := range 1000 {
		err := pool.Append(Record{
			Identity:  "worker",
			Action:    "load-test",
			Timestamp: time.Now(),
			Labels:    map[string]string{"i": time.Now().String()},
			Payload:   []byte{byte(i % 256)},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	t.Logf("1000 appends in %v (%.1f/sec)", elapsed, 1000/elapsed.Seconds())

	if pool.Len() != 1000 {
		t.Errorf("expected 1000 records, got %d", pool.Len())
	}

	if err := pool.VerifyChain(); err != nil {
		t.Errorf("chain broken after 1000 appends: %v", err)
	}
}

func TestDoltPoolEmptyState(t *testing.T) {
	db := openTestDB(t)
	pool := NewDoltPool(db.DB)

	if pool.Len() != 0 {
		t.Errorf("expected 0, got %d", pool.Len())
	}

	recs := pool.Records()
	if len(recs) != 0 {
		t.Errorf("expected empty records, got %d", len(recs))
	}

	if err := pool.VerifyChain(); err != nil {
		t.Errorf("empty chain should verify: %v", err)
	}
}

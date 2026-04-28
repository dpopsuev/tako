package depo

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/store"
)

func TestDoltShelfRejectsBrokenSeal(t *testing.T) {
	db := openTestDB(t)
	_ = NewDoltDepo(db.DB, "test")

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-tampered"
	env.Seal()
	env.Labels["injected"] = "evil"

	if env.Verify() {
		t.Error("tampered envelope should fail Verify")
	}
}

func TestDoltShelfSQLInjectionShelfName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sqldb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dp := NewDoltDepo(db.DB, "test")
	malicious := "intake'; DROP TABLE envelopes; --"
	shelf := dp.Shelf(malicious)

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-inject"
	err = shelf.Push(env)
	if err != nil {
		t.Logf("push with malicious shelf name returned error (acceptable): %v", err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM envelopes"); err != nil {
		t.Fatalf("envelopes table should still exist: %v", err)
	}
}

func TestDoltShelfSQLInjectionEnvelopeID(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "'; DROP TABLE envelopes; --"
	err := shelf.Push(env)
	if err != nil {
		t.Logf("push with malicious ID returned error (acceptable): %v", err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM envelopes"); err != nil {
		t.Fatalf("envelopes table should still exist: %v", err)
	}
}

func TestDoltShelfOversizedPayload(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("big")

	payload := []byte(strings.Repeat("x", 1024*1024))
	env := artifact.NewEnvelope("origin", payload)
	env.ID = "env-1mb"

	err := shelf.Push(env)
	if err != nil {
		t.Fatalf("1MB push failed: %v", err)
	}

	items := shelf.Peek()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Payload) != 1024*1024 {
		t.Errorf("expected 1MB payload, got %d bytes", len(items[0].Payload))
	}
}

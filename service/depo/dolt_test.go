package depo

import (
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/artifact"
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

func TestDoltShelfPushAndPeek(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	env := artifact.NewEnvelope("origin", []byte("hello"))
	env.ID = "env-1"
	env.Seal()

	if err := shelf.Push(env); err != nil {
		t.Fatalf("push: %v", err)
	}

	items := shelf.Peek()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "env-1" {
		t.Errorf("expected ID env-1, got %q", items[0].ID)
	}
	if string(items[0].Payload) != "hello" {
		t.Errorf("expected payload 'hello', got %q", items[0].Payload)
	}
}

func TestDoltShelfPullClaimsEnvelope(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-2"
	env.Seal()
	_ = shelf.Push(env)

	pulled, err := shelf.Pull("agent-1")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if pulled.ID != "env-2" {
		t.Errorf("expected ID env-2, got %q", pulled.ID)
	}

	remaining := shelf.Peek()
	if len(remaining) != 0 {
		t.Errorf("expected 0 unclaimed after pull, got %d", len(remaining))
	}
}

func TestDoltShelfPullEmptyReturnsError(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("empty")

	_, err := shelf.Pull("agent-1")
	if err == nil {
		t.Fatal("expected error on empty shelf pull")
	}
}

func TestDoltShelfLabelsPreserved(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-3"
	env.Labels["station"] = "triage"
	env.Labels["priority"] = "high"
	env.Seal()
	_ = shelf.Push(env)

	items := shelf.Peek()
	if items[0].Labels["station"] != "triage" {
		t.Errorf("expected label station=triage, got %q", items[0].Labels["station"])
	}
	if items[0].Labels["priority"] != "high" {
		t.Errorf("expected label priority=high, got %q", items[0].Labels["priority"])
	}
}

func TestDoltShelfHashPreserved(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-4"
	env.Seal()
	originalHash := env.Hash
	_ = shelf.Push(env)

	items := shelf.Peek()
	if items[0].Hash != originalHash {
		t.Errorf("hash not preserved: expected %q, got %q", originalHash, items[0].Hash)
	}
}

func TestDoltShelfWatch(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")
	shelf := dp.Shelf("intake")

	ch := shelf.Watch()

	env := artifact.NewEnvelope("origin", []byte("watch-test"))
	env.ID = "env-5"
	_ = shelf.Push(env)

	select {
	case received := <-ch:
		if received.ID != "env-5" {
			t.Errorf("expected env-5, got %q", received.ID)
		}
	default:
		t.Error("expected envelope on watch channel")
	}
}

func TestDoltShelfMultipleShelves(t *testing.T) {
	db := openTestDB(t)
	dp := NewDoltDepo(db.DB, "test")

	intake := dp.Shelf("intake")
	terminus := dp.Shelf("terminus")

	env1 := artifact.NewEnvelope("origin", []byte("a"))
	env1.ID = "env-a"
	_ = intake.Push(env1)

	env2 := artifact.NewEnvelope("origin", []byte("b"))
	env2.ID = "env-b"
	_ = terminus.Push(env2)

	if len(intake.Peek()) != 1 {
		t.Error("intake should have 1")
	}
	if len(terminus.Peek()) != 1 {
		t.Error("terminus should have 1")
	}
}

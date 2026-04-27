package depo

import (
	"errors"
	"testing"

	"github.com/dpopsuev/origami/artifact"
)

func TestStubShelfPushAndPull(t *testing.T) {
	d := NewStubDepo("test")
	shelf := d.Shelf("station-1")

	env := artifact.NewEnvelope("origin", []byte("payload"))
	env.ID = "env-1"

	if err := shelf.Push(env); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	items := shelf.Peek()
	if len(items) != 1 {
		t.Fatalf("expected 1 item on shelf, got %d", len(items))
	}

	pulled, err := shelf.Pull("agent-1")
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	if pulled.ID != "env-1" {
		t.Errorf("expected env-1, got %s", pulled.ID)
	}

	items = shelf.Peek()
	if len(items) != 0 {
		t.Errorf("expected empty shelf after pull, got %d", len(items))
	}
}

func TestStubShelfPullEmpty(t *testing.T) {
	d := NewStubDepo("test")
	shelf := d.Shelf("empty")

	_, err := shelf.Pull("agent-1")
	if !errors.Is(err, ErrShelfEmpty) {
		t.Errorf("expected ErrShelfEmpty, got %v", err)
	}
}

func TestStubShelfWatch(t *testing.T) {
	d := NewStubDepo("test")
	shelf := d.Shelf("watched")

	ch := shelf.Watch()

	env := artifact.NewEnvelope("origin", []byte("data"))
	env.ID = "env-2"
	_ = shelf.Push(env)

	received := <-ch
	if received.ID != "env-2" {
		t.Errorf("expected env-2 from watch, got %s", received.ID)
	}
}

func TestStubDepoShelvesAutoCreate(t *testing.T) {
	d := NewStubDepo("test")
	_ = d.Shelf("a")
	_ = d.Shelf("b")
	_ = d.Shelf("a") // same shelf returned

	shelves := d.Shelves()
	if len(shelves) != 2 {
		t.Errorf("expected 2 shelves, got %d", len(shelves))
	}
}

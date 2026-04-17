package stubs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/testkit/stubs"
	"github.com/dpopsuev/troupe/identity"
)

func TestStubWalker_CannedArtifact(t *testing.T) {
	art := &testArt{val: "result"}
	w := stubs.NewStubWalker("w1", map[string]circuit.Artifact{
		"recall": art,
	})

	got, err := w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got != art {
		t.Errorf("got %v, want canned artifact", got)
	}
}

func TestStubWalker_DefaultArtifact(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)

	got, err := w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Error("expected default artifact, got nil")
	}
	if got.Type() != "w1" {
		t.Errorf("Type() = %q, want %q", got.Type(), "w1")
	}
}

func TestStubWalker_ErrorInjection(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)
	injected := errors.New("boom")
	w.SetError(injected)

	_, err := w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	if !errors.Is(err, injected) {
		t.Errorf("got %v, want injected error", err)
	}
}

func TestStubWalker_Visited(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)
	w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	w.Handle(context.Background(), &stubNode{name: "triage"}, circuit.NodeContext{})
	w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})

	visited := w.Visited()
	if len(visited) != 3 {
		t.Fatalf("got %d visited, want 3", len(visited))
	}
	if visited[0] != "recall" || visited[1] != "triage" || visited[2] != "recall" {
		t.Errorf("visited = %v, want [recall triage recall]", visited)
	}
}

func TestStubWalker_Identity(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)

	id := w.Identity()
	if id.Name != "w1" {
		t.Errorf("Name = %q, want %q", id.Name, "w1")
	}
}

func TestStubWalker_SetIdentity(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)
	newID := identity.Archetype{Name: "w2"}
	w.SetIdentity(&newID)

	id := w.Identity()
	if id.Name != "w2" {
		t.Errorf("Name = %q, want %q", id.Name, "w2")
	}
}

func TestStubWalker_State(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)

	state := w.State()
	if state == nil {
		t.Fatal("State() returned nil")
	}
	if state.ID != "w1" {
		t.Errorf("State.ID = %q, want %q", state.ID, "w1")
	}
	if state.Status != "running" {
		t.Errorf("State.Status = %q, want %q", state.Status, "running")
	}
}

func TestStubWalker_Reset(t *testing.T) {
	w := stubs.NewStubWalker("w1", nil)
	w.SetError(errors.New("e"))
	w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	w.Reset()

	if len(w.Visited()) != 0 {
		t.Error("visited not cleared after Reset")
	}
	_, err := w.Handle(context.Background(), &stubNode{name: "recall"}, circuit.NodeContext{})
	if err != nil {
		t.Error("error not cleared after Reset")
	}
}

// stubNode implements circuit.Node for testing.
type stubNode struct {
	name string
}

func (n *stubNode) Name() string               { return n.name }
func (n *stubNode) Approach() identity.Element { return "" }
func (n *stubNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

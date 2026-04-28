package corpus

import (
	"errors"
	"testing"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/artifact"
)

func TestCorpusAttachAndRetrieve(t *testing.T) {
	c := New()
	c.Attach(organ.NewStubOrgan("monologue"))
	c.Attach(organ.NewStubOrgan("workstation"))

	o, err := c.Organ("monologue")
	if err != nil {
		t.Fatalf("Organ failed: %v", err)
	}
	if o.Name() != "monologue" {
		t.Errorf("expected monologue, got %s", o.Name())
	}

	organs := c.Organs()
	if len(organs) != 2 {
		t.Errorf("expected 2 organs, got %d", len(organs))
	}
}

func TestCorpusOrganNotFound(t *testing.T) {
	c := New()
	_, err := c.Organ("missing")
	if !errors.Is(err, ErrOrganNotFound) {
		t.Errorf("expected ErrOrganNotFound, got %v", err)
	}
}

func TestCorpusRoute(t *testing.T) {
	c := New()
	stub := organ.NewStubOrgan("kanban")
	c.Attach(stub)

	wire := artifact.Wire{Kind: "kanban", Payload: []byte("update")}
	if err := c.Route(wire); err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	received := stub.Received()
	if len(received) != 1 {
		t.Fatalf("expected 1 received wire, got %d", len(received))
	}
	if string(received[0].Payload) != "update" {
		t.Errorf("expected payload 'update', got %q", received[0].Payload)
	}
}

func TestCorpusRouteUnknownOrgan(t *testing.T) {
	c := New()
	wire := artifact.Wire{Kind: "nonexistent", Payload: []byte("data")}
	err := c.Route(wire)
	if !errors.Is(err, ErrOrganNotFound) {
		t.Errorf("expected ErrOrganNotFound, got %v", err)
	}
}

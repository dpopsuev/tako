package corpus

import (
	"errors"
	"testing"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/artifact"
)

func TestCorpusAttachAndRetrieve(t *testing.T) {
	c := New()
	c.Attach(organ.NewStubOrgan("monolog"))
	c.Attach(organ.NewStubOrgan("workstation"))

	o, err := c.Organ("monolog")
	if err != nil {
		t.Fatalf("Organ failed: %v", err)
	}
	if o.Name() != "monolog" {
		t.Errorf("expected monolog, got %s", o.Name())
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

func TestCorpusSubscribe_FanOut(t *testing.T) {
	c := New()
	a := organ.NewStubOrgan("andon")
	b := organ.NewStubOrgan("monolog")
	c.Attach(a)
	c.Attach(b)

	c.Subscribe("alert", "andon")
	c.Subscribe("alert", "monolog")

	wire := artifact.Wire{Kind: "alert", Payload: []byte("fire")}
	if err := c.Route(wire); err != nil {
		t.Fatalf("Route: %v", err)
	}

	if len(a.Received()) != 1 {
		t.Errorf("andon should receive 1 wire, got %d", len(a.Received()))
	}
	if len(b.Received()) != 1 {
		t.Errorf("monolog should receive 1 wire, got %d", len(b.Received()))
	}
}

func TestCorpusSubscribe_FallbackToNameMatch(t *testing.T) {
	c := New()
	stub := organ.NewStubOrgan("kanban")
	c.Attach(stub)

	wire := artifact.Wire{Kind: "kanban", Payload: []byte("task")}
	if err := c.Route(wire); err != nil {
		t.Fatalf("Route fallback: %v", err)
	}
	if len(stub.Received()) != 1 {
		t.Errorf("expected 1 received wire via fallback, got %d", len(stub.Received()))
	}
}

func TestCorpusCerebrumIsOrgan(t *testing.T) {
	c := New()
	cerebrum := organ.NewStubOrgan(organ.CerebrumOrgan)
	c.Attach(cerebrum)

	o, err := c.Organ(organ.CerebrumOrgan)
	if err != nil {
		t.Fatalf("Organ(cerebrum): %v", err)
	}
	if o.Name() != organ.CerebrumOrgan {
		t.Errorf("expected cerebrum, got %s", o.Name())
	}

	wire := artifact.Wire{Kind: string(organ.CerebrumOrgan), Payload: []byte("need")}
	if err := c.Route(wire); err != nil {
		t.Fatalf("Route to cerebrum: %v", err)
	}
	if len(cerebrum.Received()) != 1 {
		t.Errorf("cerebrum should receive 1 wire, got %d", len(cerebrum.Received()))
	}
}

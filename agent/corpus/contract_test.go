package corpus

import (
	"errors"
	"sync"
	"testing"

	"github.com/dpopsuev/tako/artifact"
)

type stubHandler struct {
	mu       sync.Mutex
	name     string
	received []artifact.Wire
}

func newStubHandler(name string) *stubHandler { return &stubHandler{name: name} }
func (s *stubHandler) Name() string           { return s.name }
func (s *stubHandler) Receive(wire artifact.Wire) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.received = append(s.received, wire)
	return nil
}
func (s *stubHandler) Received() []artifact.Wire {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]artifact.Wire(nil), s.received...)
}

func TestCorpusAttachAndRetrieve(t *testing.T) {
	c := New()
	c.Attach(newStubHandler("monolog"))
	c.Attach(newStubHandler("dialog"))

	o, err := c.Handler("monolog")
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if o.Name() != "monolog" {
		t.Errorf("expected monolog, got %s", o.Name())
	}

	handlers := c.Handlers()
	if len(handlers) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(handlers))
	}
}

func TestCorpusHandlerNotFound(t *testing.T) {
	c := New()
	_, err := c.Handler("missing")
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Errorf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestCorpusRoute(t *testing.T) {
	c := New()
	stub := newStubHandler("kanban")
	c.Attach(stub)

	wire := artifact.Wire{Kind: "kanban", Payload: []byte("update")}
	if err := c.Route(wire); err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	received := stub.Received()
	if len(received) != 1 {
		t.Fatalf("expected 1 received wire, got %d", len(received))
	}
}

func TestCorpusRouteUnknown(t *testing.T) {
	c := New()
	wire := artifact.Wire{Kind: "nonexistent", Payload: []byte("data")}
	err := c.Route(wire)
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Errorf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestCorpusSubscribe_FanOut(t *testing.T) {
	c := New()
	a := newStubHandler("andon")
	b := newStubHandler("monolog")
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

package depo

import (
	"errors"
	"sync"
	"time"

	"github.com/dpopsuev/tako/artifact"
)

var (
	ErrShelfEmpty     = errors.New("depo: shelf is empty")
	ErrAlreadyClaimed = errors.New("depo: envelope already claimed")
)

// Claim tracks who pulled an Envelope and when the lease expires.
type Claim struct {
	AgentID   string
	ExpiresAt time.Time
}

// Shelf is a named queue of Envelopes with lease-based Pull.
type Shelf interface {
	Push(envelope artifact.Envelope) error
	Pull(agentID string) (artifact.Envelope, error)
	Peek() []artifact.Envelope
	Watch() <-chan artifact.Envelope
}

// Depo is a named collection of Shelves.
type Depo interface {
	Shelf(name string) Shelf
	Shelves() []string
}

// StubDepo is an in-memory Depo.
type StubDepo struct {
	mu      sync.Mutex
	name    string
	shelves map[string]*StubShelf
}

var _ Depo = (*StubDepo)(nil)

func NewStubDepo(name string) *StubDepo {
	return &StubDepo{
		name:    name,
		shelves: make(map[string]*StubShelf),
	}
}

func (d *StubDepo) Shelf(name string) Shelf {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.shelves[name]
	if !ok {
		s = &StubShelf{name: name}
		d.shelves[name] = s
	}
	return s
}

func (d *StubDepo) Shelves() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.shelves))
	for name := range d.shelves {
		out = append(out, name)
	}
	return out
}

// StubShelf is an in-memory Shelf with lease-based Pull.
type StubShelf struct {
	mu       sync.Mutex
	name     string
	items    []artifact.Envelope
	claims   map[string]Claim
	watchers []chan artifact.Envelope
}

func (s *StubShelf) Push(envelope artifact.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, envelope)
	for _, w := range s.watchers {
		select {
		case w <- envelope:
		default:
		}
	}
	return nil
}

func (s *StubShelf) Pull(agentID string) (artifact.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if s.claims == nil {
			s.claims = make(map[string]Claim)
		}
		if _, claimed := s.claims[item.ID]; !claimed {
			s.claims[item.ID] = Claim{AgentID: agentID, ExpiresAt: time.Now().Add(5 * time.Minute)}
			s.items = append(s.items[:i], s.items[i+1:]...)
			return item, nil
		}
	}
	return artifact.Envelope{}, ErrShelfEmpty
}

func (s *StubShelf) Peek() []artifact.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]artifact.Envelope(nil), s.items...)
}

func (s *StubShelf) Watch() <-chan artifact.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan artifact.Envelope, 16)
	s.watchers = append(s.watchers, ch)
	return ch
}

package corpus

import (
	"errors"
	"sync"

	"github.com/dpopsuev/tako/artifact"
)

var (
	ErrHandlerNotFound = errors.New("corpus: handler not found")
)

// Handler is a named wire receiver. Stations, services, listeners
// implement this to receive routed wires.
type Handler interface {
	Name() string
	Receive(wire artifact.Wire) error
}

// Corpus wires Cerebrum to buses and routes wires to handlers.
type Corpus struct {
	mu            sync.RWMutex
	handlers      map[string]Handler
	subscriptions map[string][]string
}

func New() *Corpus {
	return &Corpus{
		handlers:      make(map[string]Handler),
		subscriptions: make(map[string][]string),
	}
}

func (c *Corpus) Attach(h Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[h.Name()] = h
}

func (c *Corpus) Handler(name string) (Handler, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.handlers[name]
	if !ok {
		return nil, ErrHandlerNotFound
	}
	return h, nil
}

func (c *Corpus) Handlers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.handlers))
	for name := range c.handlers {
		out = append(out, name)
	}
	return out
}

func (c *Corpus) Subscribe(kind string, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[kind] = append(c.subscriptions[kind], name)
}

func (c *Corpus) Route(wire artifact.Wire) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := c.subscriptions[wire.Kind]
	if len(subs) > 0 {
		var firstErr error
		for _, name := range subs {
			if h, ok := c.handlers[name]; ok {
				if err := h.Receive(wire); err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	}

	h, ok := c.handlers[wire.Kind]
	if !ok {
		return ErrHandlerNotFound
	}
	return h.Receive(wire)
}

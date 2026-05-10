package corpus

import (
	"errors"
	"sync"

	"github.com/dpopsuev/tako/agent/organ"
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

// Corpus is the agent's body — registers all organs (built-in + environment),
// wires buses, enforces gating. The composition root.
type Corpus struct {
	mu            sync.RWMutex
	handlers      map[string]Handler
	organs  *organ.FuncSet
	subscriptions map[string][]string
}

func New() *Corpus {
	return &Corpus{
		handlers:      make(map[string]Handler),
		organs:  organ.NewFuncSet(),
		subscriptions: make(map[string][]string),
	}
}

// Register adds a Organ to the Corpus. The unified path —
// no distinction between organ, instrument, or organ.
func (c *Corpus) Register(cap organ.Func) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.organs.Register(cap)
}

// Organ returns a registered capability by name.
func (c *Corpus) Organ(name string) (organ.Func, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.organs.Get(name)
}

// Organs returns the full set.
func (c *Corpus) Organs() *organ.FuncSet {
	return c.organs
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


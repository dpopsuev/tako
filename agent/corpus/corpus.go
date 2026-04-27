package corpus

import (
	"errors"
	"sync"

	"github.com/dpopsuev/origami/agent/organ"
	"github.com/dpopsuev/origami/artifact"
)

var (
	ErrOrganNotFound = errors.New("corpus: organ not found")
	ErrNotAssembled  = errors.New("corpus: not assembled")
)

// Corpus is the agent's body — a collection of Organs assembled from AAI.Capability.
// Tangled builds the Corpus. Agent never self-assembles.
type Corpus struct {
	mu     sync.RWMutex
	organs map[string]organ.Organ
}

// New creates an empty Corpus. Organs are attached via Attach.
func New() *Corpus {
	return &Corpus{organs: make(map[string]organ.Organ)}
}

// Attach adds an Organ to the Corpus.
func (c *Corpus) Attach(o organ.Organ) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.organs[o.Name()] = o
}

// Organ returns a named Organ.
func (c *Corpus) Organ(name string) (organ.Organ, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.organs[name]
	if !ok {
		return nil, ErrOrganNotFound
	}
	return o, nil
}

// Organs returns all attached Organ names.
func (c *Corpus) Organs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.organs))
	for name := range c.organs {
		out = append(out, name)
	}
	return out
}

// Route dispatches a Wire to the Organ matching Wire.Kind.
func (c *Corpus) Route(wire artifact.Wire) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.organs[wire.Kind]
	if !ok {
		return ErrOrganNotFound
	}
	return o.Receive(wire)
}

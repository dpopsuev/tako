package corpus

import (
	"context"
	"errors"
	"sync"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/artifact"
)

var (
	ErrOrganNotFound = errors.New("corpus: organ not found")
	ErrNotAssembled  = errors.New("corpus: not assembled")
)

// Cerebrum is the agent's mind — thinking, memory, LLM access.
type Cerebrum interface {
	Think(ctx context.Context, need []byte) error
}

// Corpus is the composition root — wires Cerebrum (mind) and Organs (body).
// Tangled builds the Corpus. Agent never self-assembles. SOLID DIP.
type Corpus struct {
	mu       sync.RWMutex
	cerebrum Cerebrum
	organs   map[organ.OrganName]organ.Organ
}

// New creates an empty Corpus. Cerebrum and Organs attached via setters.
func New() *Corpus {
	return &Corpus{organs: make(map[organ.OrganName]organ.Organ)}
}

// SetCerebrum attaches the mind.
func (c *Corpus) SetCerebrum(cerebrum Cerebrum) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cerebrum = cerebrum
}

// Cerebrum returns the mind.
func (c *Corpus) GetCerebrum() Cerebrum {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cerebrum
}

// Attach adds an Organ to the Corpus.
func (c *Corpus) Attach(o organ.Organ) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.organs[o.Name()] = o
}

// Organ returns a named Organ.
func (c *Corpus) Organ(name organ.OrganName) (organ.Organ, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.organs[name]
	if !ok {
		return nil, ErrOrganNotFound
	}
	return o, nil
}

// Organs returns all attached Organ names.
func (c *Corpus) Organs() []organ.OrganName {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]organ.OrganName, 0, len(c.organs))
	for name := range c.organs {
		out = append(out, name)
	}
	return out
}

// Route dispatches a Wire to the Organ matching Wire.Kind.
func (c *Corpus) Route(wire artifact.Wire) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.organs[organ.OrganName(wire.Kind)]
	if !ok {
		return ErrOrganNotFound
	}
	return o.Receive(wire)
}

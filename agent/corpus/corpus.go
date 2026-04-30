package corpus

import (
	"errors"
	"sync"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/artifact"
)

var (
	ErrOrganNotFound = errors.New("corpus: organ not found")
	ErrNotAssembled  = errors.New("corpus: not assembled")
)

// Corpus is the composition root — wires Organs (body + mind).
// Cerebrum IS an Organ — same interface, attached via Attach().
// Tangled builds the Corpus. Agent never self-assembles. SOLID DIP.
type Corpus struct {
	mu            sync.RWMutex
	organs        map[organ.OrganName]organ.Organ
	subscriptions map[string][]organ.OrganName
}

// New creates an empty Corpus. Cerebrum and Organs attached via setters.
func New() *Corpus {
	return &Corpus{
		organs:        make(map[organ.OrganName]organ.Organ),
		subscriptions: make(map[string][]organ.OrganName),
	}
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

// Subscribe registers an Organ to receive Wires of a given kind.
func (c *Corpus) Subscribe(kind string, name organ.OrganName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[kind] = append(c.subscriptions[kind], name)
}

// Route dispatches a Wire to all Organs subscribed to Wire.Kind.
// Falls back to OrganName matching if no subscriptions exist.
func (c *Corpus) Route(wire artifact.Wire) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := c.subscriptions[wire.Kind]
	if len(subs) > 0 {
		var firstErr error
		for _, name := range subs {
			if o, ok := c.organs[name]; ok {
				if err := o.Receive(wire); err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	}

	o, ok := c.organs[organ.OrganName(wire.Kind)]
	if !ok {
		return ErrOrganNotFound
	}
	return o.Receive(wire)
}

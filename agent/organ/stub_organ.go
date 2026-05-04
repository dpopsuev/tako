package organ

import (
	"sync"

	"github.com/dpopsuev/tako/artifact"
)

// StubOrgan is a test organ that records received wires.
type StubOrgan struct {
	mu       sync.Mutex
	name     OrganName
	received []artifact.Wire
}

var _ Organ = (*StubOrgan)(nil)

func NewStubOrgan(name OrganName) *StubOrgan {
	return &StubOrgan{name: name}
}

func (o *StubOrgan) Name() OrganName { return o.name }

func (o *StubOrgan) Receive(wire artifact.Wire) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.received = append(o.received, wire)
	return nil
}

// Received returns all wires received by this organ.
func (o *StubOrgan) Received() []artifact.Wire {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]artifact.Wire(nil), o.received...)
}

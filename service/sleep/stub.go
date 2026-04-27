package sleep

import "github.com/dpopsuev/origami/memory"

// StubDrain is a noop — accepts mesh, does nothing.
type StubDrain struct {
	SweptCount        int
	ConsolidatedCount int
}

var _ Drain = (*StubDrain)(nil)

func (d *StubDrain) Sweep(_ memory.Mesh) error {
	d.SweptCount++
	return nil
}

func (d *StubDrain) Consolidate(_ memory.Mesh) error {
	d.ConsolidatedCount++
	return nil
}

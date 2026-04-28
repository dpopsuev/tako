package finding

// Category: Processing & Support

import (
	"context"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// InMemoryFindingCollector is a thread-safe, slice-backed FindingCollector.
type InMemoryFindingCollector struct {
	mu       sync.RWMutex
	findings []circuit.Finding
}

func (c *InMemoryFindingCollector) Report(_ context.Context, f *circuit.Finding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}
	c.findings = append(c.findings, *f)
	return nil
}

func (c *InMemoryFindingCollector) Findings() []circuit.Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]circuit.Finding, len(c.findings))
	copy(out, c.findings)
	return out
}

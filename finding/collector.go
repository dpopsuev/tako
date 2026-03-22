package finding

// Category: Processing & Support

import (
	"context"
	"sync"
	"time"

	"github.com/dpopsuev/origami/core"
)

// InMemoryFindingCollector is a thread-safe, slice-backed FindingCollector.
type InMemoryFindingCollector struct {
	mu       sync.RWMutex
	findings []core.Finding
}

func (c *InMemoryFindingCollector) Report(_ context.Context, f core.Finding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}
	c.findings = append(c.findings, f)
	return nil
}

func (c *InMemoryFindingCollector) Findings() []core.Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]core.Finding, len(c.findings))
	copy(out, c.findings)
	return out
}

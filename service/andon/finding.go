package andon

import (
	"context"
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Finding struct {
	Severity  Severity       `json:"severity"`
	Domain    string         `json:"domain"`
	Source    string         `json:"source"`
	Station   string         `json:"station"`
	Message   string         `json:"message"`
	Evidence  map[string]any `json:"evidence,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type FindingCollector interface {
	Report(ctx context.Context, f *Finding) error
	Findings() []Finding
}

type InMemoryCollector struct {
	mu       sync.RWMutex
	findings []Finding
}

func (c *InMemoryCollector) Report(_ context.Context, f *Finding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}
	c.findings = append(c.findings, *f)
	return nil
}

func (c *InMemoryCollector) Findings() []Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Finding, len(c.findings))
	copy(out, c.findings)
	return out
}

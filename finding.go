package framework

// Category: Processing & Support — aliases to core/ package.
// Implementations (InMemoryFindingCollector, vetoArtifact) stay here.

import (
	"context"
	"sync"
	"time"

	"github.com/dpopsuev/origami/core"
)

type FindingSeverity = core.FindingSeverity

const (
	FindingInfo    = core.FindingInfo
	FindingWarning = core.FindingWarning
	FindingError   = core.FindingError
)

type Finding = core.Finding
type FindingCollector = core.FindingCollector

const FindingCollectorKey = core.FindingCollectorKey

func SeverityAtOrAbove(have, threshold FindingSeverity) bool {
	return core.SeverityAtOrAbove(have, threshold)
}

// InMemoryFindingCollector is a thread-safe, slice-backed FindingCollector.
type InMemoryFindingCollector struct {
	mu       sync.RWMutex
	findings []Finding
}

func (c *InMemoryFindingCollector) Report(_ context.Context, f Finding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}
	c.findings = append(c.findings, f)
	return nil
}

func (c *InMemoryFindingCollector) Findings() []Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Finding, len(c.findings))
	copy(out, c.findings)
	return out
}

// vetoArtifact wraps an artifact and overrides Confidence to 0.
// Used by the hookingWalker when a VetoHook returns ErrFindingVeto.
type vetoArtifact struct {
	inner Artifact
}

func (v *vetoArtifact) Type() string        { return v.inner.Type() }
func (v *vetoArtifact) Confidence() float64 { return 0 }
func (v *vetoArtifact) Raw() any            { return v.inner.Raw() }

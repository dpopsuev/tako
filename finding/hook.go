package finding

// Category: Processing & Support

import (
	"context"

	"github.com/dpopsuev/origami/core"
)

// VetoHook is an after-hook that checks the FindingCollector for
// FindingError findings targeting the current node. When found, it
// returns ErrFindingVeto, which the hookingWalker intercepts to wrap
// the artifact with Confidence() 0.
type VetoHook struct {
	collector core.FindingCollector
}

// NewVetoHook creates a VetoHook backed by the given collector.
func NewVetoHook(collector core.FindingCollector) *VetoHook {
	return &VetoHook{collector: collector}
}

func (h *VetoHook) Name() string { return "finding-veto" }

func (h *VetoHook) Run(_ context.Context, nodeName string, artifact core.Artifact) error {
	if artifact == nil {
		return nil
	}
	for _, f := range h.collector.Findings() {
		if f.Severity == core.FindingError && f.NodeName == nodeName {
			return core.ErrFindingVeto
		}
	}
	return nil
}

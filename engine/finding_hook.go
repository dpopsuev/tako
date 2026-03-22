package engine

// Category: Processing & Support

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
)

// VetoHook is an after-hook that checks the FindingCollector for
// FindingError findings targeting the current node. When found, it
// returns ErrFindingVeto, which the hookingWalker intercepts to wrap
// the artifact with Confidence() 0.
type VetoHook struct {
	collector circuit.FindingCollector
}

// NewVetoHook creates a VetoHook backed by the given collector.
func NewVetoHook(collector circuit.FindingCollector) *VetoHook {
	return &VetoHook{collector: collector}
}

func (h *VetoHook) Name() string { return "finding-veto" }

func (h *VetoHook) Run(_ context.Context, nodeName string, artifact circuit.Artifact) error {
	if artifact == nil {
		return nil
	}
	for _, f := range h.collector.Findings() {
		if f.Severity == circuit.FindingError && f.NodeName == nodeName {
			return circuit.ErrFindingVeto
		}
	}
	return nil
}

package arcade

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/artifact"
)

// HITLListener is a Signal organ that watches for pending.hitl wires
// and responds with approval.hitl events on the sensory bus.
// Involuntary — the agent doesn't control when approval arrives.
type HITLListener struct {
	sensory cerebrum.Bus
	approve func(wire artifact.Wire) bool
}

func NewHITLListener(sensory cerebrum.Bus, approve func(artifact.Wire) bool) *HITLListener {
	return &HITLListener{sensory: sensory, approve: approve}
}

func (h *HITLListener) Name() string { return "hitl" }

func (h *HITLListener) Receive(wire artifact.Wire) error {
	if wire.Kind != "motor.pending.hitl" {
		return nil
	}
	if h.approve(wire) {
		h.sensory.Send(context.Background(), cerebrum.Event{
			ID:        fmt.Sprintf("hitl-approve-%d", time.Now().UnixNano()),
			Kind:      "approval.hitl",
			Source:    "human",
			Payload:   wire.Payload,
			CreatedAt: time.Now(),
		})
	}
	return nil
}

// AutoApproveHITL returns a listener that approves everything.
func AutoApproveHITL(sensory cerebrum.Bus) *HITLListener {
	return NewHITLListener(sensory, func(_ artifact.Wire) bool { return true })
}

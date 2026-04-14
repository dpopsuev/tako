package operator

import (
	"time"

	"github.com/dpopsuev/troupe/world"
)

// OperatorRole is the canonical role for the human operator entity.
const OperatorRole = "operator"

// RegisterOperator spawns a reactive World entity for the human operator.
// The operator is NOT a full Actor (no LLM process, no Perform method).
// It exists for discoverability (Display), readiness (Ready), and edge
// wiring (CommunicatesWith) so agents can address the human.
func RegisterOperator(w *world.World, name string) world.EntityID {
	id := w.Spawn()

	world.Attach(w, id, world.Display{
		Name: name,
		Icon: "operator",
	})

	world.Attach(w, id, world.Ready{
		Ready:    true,
		LastSeen: time.Now(),
	})

	return id
}

// WireOperatorEdges creates CommunicatesWith edges between the operator
// and each agent. Bidirectional — agents can address the operator and
// the operator can address agents.
func WireOperatorEdges(w *world.World, operatorID world.EntityID, agentIDs []world.EntityID) error {
	for _, agentID := range agentIDs {
		if err := w.Link(operatorID, world.CommunicatesWith, agentID); err != nil {
			return err
		}
		if err := w.Link(agentID, world.CommunicatesWith, operatorID); err != nil {
			return err
		}
	}
	return nil
}

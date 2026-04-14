package operator_test

import (
	"testing"

	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/troupe/world"
)

func TestRegisterOperator_SpawnsEntityWithComponents(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()

	id := operator.RegisterOperator(w, "Alice")

	if !w.Alive(id) {
		t.Fatal("operator entity not alive")
	}

	display, ok := world.TryGet[world.Display](w, id)
	if !ok {
		t.Fatal("Display component missing")
	}
	if display.Name != "Alice" {
		t.Errorf("Display.Name = %q, want Alice", display.Name)
	}

	ready, ok := world.TryGet[world.Ready](w, id)
	if !ok {
		t.Fatal("Ready component missing")
	}
	if !ready.Ready {
		t.Error("Ready.Ready = false, want true")
	}
}

func TestRegisterOperator_Discoverable(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()

	opID := operator.RegisterOperator(w, "Bob")

	// Query all entities with Display — operator should appear.
	entities := world.Query[world.Display](w)
	found := false
	for _, id := range entities {
		if id == opID {
			found = true
			break
		}
	}
	if !found {
		t.Error("operator not discoverable via Display query")
	}
}

func TestWireOperatorEdges_BidirectionalCommunication(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()

	opID := operator.RegisterOperator(w, "Operator")
	agent1 := w.Spawn()
	agent2 := w.Spawn()

	err := operator.WireOperatorEdges(w, opID, []world.EntityID{agent1, agent2})
	if err != nil {
		t.Fatalf("WireOperatorEdges: %v", err)
	}

	// Operator → agents (outbound).
	neighbors := w.Neighbors(opID, world.CommunicatesWith, world.Outbound)
	if len(neighbors) != 2 {
		t.Errorf("operator outbound neighbors = %d, want 2", len(neighbors))
	}

	// Agent1 → operator (outbound).
	agentNeighbors := w.Neighbors(agent1, world.CommunicatesWith, world.Outbound)
	found := false
	for _, n := range agentNeighbors {
		if n == opID {
			found = true
			break
		}
	}
	if !found {
		t.Error("agent1 cannot address operator (missing CommunicatesWith edge)")
	}
}

func TestWireOperatorEdges_EmptyAgentList(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()

	opID := operator.RegisterOperator(w, "Solo")
	err := operator.WireOperatorEdges(w, opID, nil)
	if err != nil {
		t.Fatalf("WireOperatorEdges with nil: %v", err)
	}
}

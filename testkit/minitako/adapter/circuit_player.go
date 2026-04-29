package adapter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/testkit/minitako"
)

type CircuitPlayer struct {
	cerebrum *cerebrum.Cerebrum
}

func NewCircuitPlayer(cb *cerebrum.Cerebrum) *CircuitPlayer {
	return &CircuitPlayer{cerebrum: cb}
}

func (p *CircuitPlayer) Act(state minitako.GameState) minitako.Action {
	stateJSON, _ := json.Marshal(state)
	need := append([]byte("Minitako game state: "), stateJSON...)

	if err := p.cerebrum.Think(context.Background(), need); err != nil {
		return minitako.Feed
	}

	m := p.cerebrum.Result()
	if m == nil {
		return minitako.Feed
	}

	return extractAction(m, &state)
}

func extractAction(m *reactivity.Molecule, state *minitako.GameState) minitako.Action {
	for _, a := range m.Atoms(reactivity.ExecutionAtom) {
		content := strings.ToLower(string(a.Content))
		for _, action := range minitako.AvailableActions(state) {
			if strings.Contains(content, action.String()) {
				return action
			}
		}
	}

	for _, a := range m.Atoms(reactivity.PlanAtom) {
		content := strings.ToLower(string(a.Content))
		for _, action := range minitako.AvailableActions(state) {
			if strings.Contains(content, action.String()) {
				return action
			}
		}
	}

	return minitako.Feed
}

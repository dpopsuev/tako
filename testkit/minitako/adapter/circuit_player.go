package adapter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/testkit/minitako"
)

type ReactorPlayer struct {
	cerebrum *cerebrum.Cerebrum
}

func NewReactorPlayer(cb *cerebrum.Cerebrum) *ReactorPlayer {
	return &ReactorPlayer{cerebrum: cb}
}

func (p *ReactorPlayer) Act(state minitako.GameState) minitako.Action {
	stateJSON, _ := json.Marshal(state)

	if _, err := p.cerebrum.Think(context.Background(), reactivity.Catalyst{Need: "Minitako game state: " + string(stateJSON)}); err != nil {
		return minitako.Feed
	}

	m := p.cerebrum.Result()
	if m == nil {
		return minitako.Feed
	}

	return extractAction(m, &state)
}

func extractAction(m *reactivity.Molecule, state *minitako.GameState) minitako.Action {
	for _, phase := range []reactivity.AtomType{
		reactivity.ExecutionAtom,
		reactivity.SelectionAtom,
		reactivity.ExpansionAtom,
		reactivity.RefinementAtom,
	} {
		for _, a := range m.Atoms(phase) {
			content := strings.ToLower(string(a.Content))
			for _, action := range minitako.AvailableActions(state) {
				if strings.Contains(content, action.String()) {
					return action
				}
			}
		}
	}
	return minitako.Feed
}

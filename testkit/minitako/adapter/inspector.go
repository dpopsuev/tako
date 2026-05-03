package adapter

import (
	"time"

	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/testkit/minitako"
)

type ErgographInspector struct {
	pool    ergograph.Ledger
	optimal minitako.GameInspector
}

func NewErgographInspector(pool ergograph.Ledger) *ErgographInspector {
	return &ErgographInspector{
		pool:    pool,
		optimal: minitako.StubInspector{},
	}
}

func (e *ErgographInspector) Score(state minitako.GameState, action minitako.Action) float64 {
	opt := e.optimal.OptimalAction(state)
	score := 0.5
	if action == opt {
		score = 1.0
	}

	e.pool.Append(ergograph.Record{
		Identity:  "minitako-inspector",
		Action:    "score",
		Timestamp: time.Now(),
		Labels: map[string]string{
			"action":  action.String(),
			"optimal": opt.String(),
			"stage":   state.Stage.String(),
		},
		Payload: []byte{},
	})

	return score
}

func (e *ErgographInspector) OptimalAction(state minitako.GameState) minitako.Action {
	return e.optimal.OptimalAction(state)
}

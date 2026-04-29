package minitako

import (
	"encoding/json"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/fab"
)

type DayContract struct {
	MaxActions int
}

func NewDayContract() *DayContract {
	return &DayContract{MaxActions: 14}
}

func (c *DayContract) Evaluate(_ fab.Contract, envelope artifact.Envelope) (bool, error) {
	var gs GameState
	if err := json.Unmarshal(envelope.Payload, &gs); err != nil {
		return false, err
	}
	return gs.ActionTicker >= c.MaxActions, nil
}

type NightContract struct{}

func NewNightContract() *NightContract {
	return &NightContract{}
}

func (c *NightContract) Evaluate(_ fab.Contract, _ artifact.Envelope) (bool, error) {
	return true, nil
}

func PackState(gs *GameState) artifact.Envelope {
	payload, _ := json.Marshal(gs)
	env := artifact.NewEnvelope("minitako", payload)
	env.Labels["game"] = "minitako"
	env.Labels["day"] = string(rune('0' + gs.Day))
	env.Labels["stage"] = gs.Stage.String()
	env.Seal()
	return env
}

func UnpackState(env artifact.Envelope) (GameState, error) {
	var gs GameState
	err := json.Unmarshal(env.Payload, &gs)
	return gs, err
}

package minitako

import (
	"encoding/json"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/fab"
)

var (
	DayDone = fab.MustPredicate(`output.ActionTicker <= 0`)
	PetDied = fab.MustPredicate(`output.Alive == false`)
)

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

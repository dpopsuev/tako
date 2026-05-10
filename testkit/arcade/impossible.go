package arcade

import (
	"encoding/json"

	"github.com/dpopsuev/tako/agent/organ"
)

func NewImpossible() Scenario {
	adv := NewGame(map[string]any{
		"locked": true,
		"key":    "hidden",
	})

	adv.Organ("look", "Look at the locked door", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["locked"] == true {
				return organ.TextResult("you see a locked door. it requires a key to open. you need to use the 'unlock' instrument."), nil
			}
			return organ.TextResult("the door is open"), nil
		})

	return Scenario{
		Name:         "impossible",
		Need:         "Open the locked door. You must unlock it with a key.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["locked"] == false },
		OptimalTurns: 1,
	}
}

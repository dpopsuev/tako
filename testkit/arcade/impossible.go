package arcade

import (
	"github.com/dpopsuev/tako/agent/organ"
)

// NewImpossible returns a scenario where the Need requires actions
// the agent cannot perform — the only action is read-only (look).
// Think should detect impossibility: Distance stays 1.0, Momentum
// stays 0, Assert fires Subcritical → SCRAM.
func NewImpossible() Scenario {
	adv := NewGame(map[string]any{
		"locked": true,
		"key":    "hidden",
	})

	adv.AddInstrument("look", "Look at the locked door", organ.ReadAction, func(s map[string]any, _ string) string {
		if s["locked"] == true {
			return "you see a locked door. it requires a key to open. you need to use the 'unlock' instrument."
		}
		return "the door is open"
	})

	return Scenario{
		Name:         "impossible",
		Need:         "Open the locked door. You must unlock it with a key.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["locked"] == false },
		OptimalTurns: 1,
	}
}

package arcade

import (
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/testkit/rehearsal"
	tangle "github.com/dpopsuev/tangle"
)

func BuildArcadeAgent(scenario Scenario, completer tangle.Completer) *assemble.Agent {
	bp := assemble.Blueprint{
		Model:        "arcade",
		Capabilities: scenario.Adventure.Capabilities(),
		Budget: cerebrum.Budget{
			MaxTurns:    30,
			TurnTimeout: 30 * time.Second,
		},
	}
	return assemble.Assemble(bp, completer)
}

func ArcadeReferee(scenario Scenario) *rehearsal.GameReferee {
	return &rehearsal.GameReferee{
		IsSolved: scenario.IsSolved,
		State:    scenario.Adventure.State,
	}
}

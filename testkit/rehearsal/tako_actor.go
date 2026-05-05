package rehearsal

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/shells/code"
	tangle "github.com/dpopsuev/tangle"
)

func BlueprintActorFactory(completer tangle.Completer) ActorFactory {
	return func(workspace string) (ActorFunc, error) {
		bp := assemble.Blueprint{
			Model:        "rehearsal",
			Capabilities: code.Capabilities(workspace),
			Budget: cerebrum.Budget{
				MaxTurns:    15,
				TurnTimeout: 30 * time.Second,
			},
		}
		agent := assemble.Assemble(bp, completer)

		return func(ctx context.Context, prompt string) (string, error) {
			err := agent.Think(ctx, prompt)
			if err != nil {
				return "", err
			}
			m := agent.Result()
			return extractResult(m), nil
		}, nil
	}
}

func extractResult(m *reactivity.Molecule) string {
	retro := m.ByTaxonomy("retrospection.")
	if len(retro) > 0 {
		return string(retro[len(retro)-1].Content)
	}
	return fmt.Sprintf("completed (mass=%d, distance=%.2f)", m.TotalMass(), m.Distance())
}

package instrument

import (
	"context"

	troupe "github.com/dpopsuev/tangle"
)

// TangleCompleter wraps a Tangle Agent.Perform into the Completer interface.
// The Agent came from Caster.Pick → Caster.Spawn. The Completer is the port
// the Cerebrum uses. This adapter bridges infrastructure (Tangle) to domain (Tako).
type TangleCompleter struct {
	agent troupe.Agent
}

var _ Completer = (*TangleCompleter)(nil)

func NewTangleCompleter(agent troupe.Agent) *TangleCompleter {
	return &TangleCompleter{agent: agent}
}

func (tc *TangleCompleter) Complete(ctx context.Context, prompt []byte) ([]byte, error) {
	result, err := tc.agent.Perform(ctx, string(prompt))
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

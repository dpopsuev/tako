package agent

import (
	"context"

	"github.com/dpopsuev/tako/agent/corpus"
)

// Agent is the runtime representation of an agent inside a Fab.
// Corpus wires Cerebrum to buses. Reactivity is the cognitive loop.
type Agent struct {
	Identity   string
	Persona    Uniform
	Corpus     *corpus.Corpus
	Reactivity Reactivity
}

// Runner executes the agent's reactivity loop at a station.
type Runner interface {
	Run(ctx context.Context, agent *Agent) error
}

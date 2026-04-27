package agent

import (
	"context"

	"github.com/dpopsuev/origami/agent/corpus"
)

// Agent is the runtime representation of an agent inside a Fab.
// Corpus is its body (Organs attached by Tangled from AAI.Capability).
// Reactivity is its brain (FSM cognitive loop).
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

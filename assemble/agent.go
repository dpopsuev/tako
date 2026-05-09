package assemble

import (
	"context"
	"fmt"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/reactivity"
)

type Agent struct {
	cerebrum *cerebrum.Cerebrum
	sensory  cerebrum.Bus
	signal   cerebrum.Bus
	corpus   *corpus.Corpus
}

func (a *Agent) Think(ctx context.Context, need string) (cerebrum.ThinkOutcome, error) {
	return a.cerebrum.Think(ctx, reactivity.Catalyst{Need: need})
}

func (a *Agent) Result() *reactivity.Molecule {
	return a.cerebrum.Result()
}

func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	if _, err := a.Think(ctx, task); err != nil {
		return "", err
	}
	m := a.Result()
	if r := m.Response(); r != "" {
		return r, nil
	}
	if chain := m.Chain(); chain != nil && chain.Len() > 0 {
		last, ok := chain.Last()
		if ok && len(last.Output) > 0 {
			return string(last.Output), nil
		}
	}
	retro := m.ByTaxonomy("retrospection.")
	if len(retro) > 0 {
		return string(retro[len(retro)-1].Content), nil
	}
	return fmt.Sprintf("completed (mass=%d, distance=%.2f)", m.TotalMass(), m.Distance()), nil
}

package assemble

import (
	"context"

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

func (a *Agent) ThinkWith(ctx context.Context, catalyst reactivity.Catalyst) (cerebrum.ThinkOutcome, error) {
	return a.cerebrum.Think(ctx, catalyst)
}

func (a *Agent) Result() *reactivity.Molecule {
	return a.cerebrum.Result()
}

func (a *Agent) LastSummary() cerebrum.SessionSummary {
	return a.cerebrum.LastSummary()
}

func (a *Agent) Run(ctx context.Context, task string) error {
	_, err := a.Think(ctx, task)
	return err
}

func (a *Agent) LastOutput() string {
	m := a.Result()
	if m == nil {
		return ""
	}
	chain := m.Chain()
	if chain == nil {
		return ""
	}
	motors := chain.Motors()
	if len(motors) == 0 {
		return ""
	}
	return string(motors[len(motors)-1].Output)
}

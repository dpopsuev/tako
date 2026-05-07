package assemble

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
	tangle "github.com/dpopsuev/tangle"
)

type Blueprint struct {
	Model        string
	ModelWatcher string
	Capabilities []organ.Func
	Budget       cerebrum.Budget
	Config       *reactivity.Config
}

type Agent struct {
	cerebrum *cerebrum.Cerebrum
	sensory  cerebrum.Bus
	signal   cerebrum.Bus
	corpus   *corpus.Corpus
}

func (a *Agent) Think(ctx context.Context, need string) error {
	return a.cerebrum.Think(ctx, reactivity.Catalyst{Need: need})
}

func (a *Agent) Result() *reactivity.Molecule {
	return a.cerebrum.Result()
}

func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	if err := a.Think(ctx, task); err != nil {
		return "", err
	}
	m := a.Result()
	if r := m.Response(); r != "" {
		return r, nil
	}
	retro := m.ByTaxonomy("retrospection.")
	if len(retro) > 0 {
		return string(retro[len(retro)-1].Content), nil
	}
	return fmt.Sprintf("completed (mass=%d, distance=%.2f)", m.TotalMass(), m.Distance()), nil
}

func Assemble(bp Blueprint, completer tangle.Completer, opts ...cerebrum.Option) *Agent {
	cfg := bp.Config
	if cfg == nil {
		cfg = &reactivity.DefaultConfig
	}

	budget := bp.Budget
	if budget.MaxTurns == 0 {
		budget = cerebrum.DefaultBudget
	}

	nav := reactivity.NewTreeNavigator(cfg)
	reactor := reactivity.NewReactor(
		reactivity.WithNavigator(nav),
	)

	sensory := cerebrum.NewChannelBus(64)
	corp := corpus.New()

	allCaps := make([]organ.Func, 0, len(bp.Capabilities)+1)
	for _, cap := range bp.Capabilities {
		corp.Register(cap)
		allCaps = append(allCaps, cap)

		slog.Info("assemble.capability",
			slog.String("name", cap.Name),
			slog.Float64("risk", cap.Risk),
			slog.String("source", cap.Source.String()))
	}

	speakCap := speakCapability()
	corp.Register(speakCap)
	allCaps = append(allCaps, speakCap)

	subagent := &SubagentFactory{Root: ".", Completer: completer}
	subCap := subagent.Capability()
	corp.Register(subCap)
	allCaps = append(allCaps, subCap)

	if len(bp.Capabilities) == 0 {
		slog.Warn("assemble.no_capabilities")
	}

	var cb *cerebrum.Cerebrum

	signal := cerebrum.NewChannelBus(64)
	motorBus := corp.MotorBus(sensory, signal, nil)

	baseOpts := []cerebrum.Option{
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(budget),
		cerebrum.WithCapabilities(allCaps),
		cerebrum.WithConfig(cfg),
	}
	cb = cerebrum.New(reactor, completer, append(baseOpts, opts...)...)

	slog.Info("assemble.complete",
		slog.String("model", bp.Model),
		slog.Int("capabilities", len(bp.Capabilities)),
		slog.Int("max_turns", budget.MaxTurns))

	return &Agent{
		cerebrum: cb,
		sensory:  sensory,
		signal:   signal,
		corpus:   corp,
	}
}

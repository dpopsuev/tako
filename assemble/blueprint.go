package assemble

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
	"github.com/jmoiron/sqlx"
)

type Blueprint struct {
	Model        string
	ModelWatcher string
	Watcher      tangle.Completer
	DoltDB       *sqlx.DB
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

	embedder := cerebrum.StubEmbedder{}

	var reflexStore cerebrum.ReflexStore
	var recorder cerebrum.Recorder
	if bp.DoltDB != nil {
		reflexStore = cerebrum.NewDoltPipeStore(bp.DoltDB)
		recorder = cerebrum.NewDoltTurnRecorder(bp.DoltDB)
	} else {
		reflexStore = cerebrum.NewPipeStore()
	}

	consolidator := &cerebrum.PipeConsolidator{
		Store:    reflexStore,
		Embedder: embedder,
	}

	baseOpts := []cerebrum.Option{
		cerebrum.WithSensory(sensory),
		cerebrum.WithSignal(signal),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(budget),
		cerebrum.WithCapabilities(allCaps),
		cerebrum.WithConfig(cfg),
		cerebrum.WithEmbedder(embedder),
		cerebrum.WithReflexStore(reflexStore),
		cerebrum.WithConsolidator(consolidator),
		cerebrum.WithRegulator(&cerebrum.DeltaRegulator{}),
		cerebrum.WithAlignmentChecker(cerebrum.TieredAlignment{}),
	}

	if recorder != nil {
		baseOpts = append(baseOpts, cerebrum.WithRecorder(recorder))
	}

	if bp.Watcher != nil {
		baseOpts = append(baseOpts, cerebrum.WithWatcher(bp.Watcher))
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

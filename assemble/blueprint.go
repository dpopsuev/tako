package assemble

import (
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
	Organs []organ.Func
	Budget       cerebrum.Budget
	Config       *reactivity.Config
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

	if len(bp.Organs) == 0 {
		slog.Warn("assemble.no_organs")
	}

	for _, cap := range bp.Organs {
		corp.Register(cap)
		slog.Info("assemble.capability",
			slog.String("name", cap.Name),
			slog.Float64("risk", cap.Risk),
			slog.String("source", cap.Source.String()))
	}

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
		cerebrum.WithOrgans(bp.Organs),
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
	cb := cerebrum.New(reactor, completer, append(baseOpts, opts...)...)

	slog.Info("assemble.complete",
		slog.String("model", bp.Model),
		slog.Int("organs", len(bp.Organs)),
		slog.Int("max_turns", budget.MaxTurns))

	return &Agent{
		cerebrum: cb,
		sensory:  sensory,
		signal:   signal,
		corpus:   corp,
	}
}

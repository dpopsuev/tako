package assemble

import (
	"context"
	"log/slog"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/shell"
	tangle "github.com/dpopsuev/tangle"
)

type Blueprint struct {
	Model        string
	Capabilities []shell.Capability
	Budget       cerebrum.Budget
	Config       *reactivity.Config
}

type Agent struct {
	Cerebrum *cerebrum.Cerebrum
	Sensory  cerebrum.Bus
	Corpus   *corpus.Corpus
}

func (a *Agent) Think(ctx context.Context, need string) error {
	return a.Cerebrum.Think(ctx, reactivity.Catalyst{Need: need})
}

func (a *Agent) Result() *reactivity.Molecule {
	return a.Cerebrum.Result()
}

func Assemble(bp Blueprint, completer tangle.Completer) *Agent {
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

	var tools []tangle.Tool
	for _, cap := range bp.Capabilities {
		corp.Register(cap)
		tools = append(tools, tangle.Tool{
			Name:        cap.Name,
			Description: cap.Description,
			InputSchema: cap.Schema,
		})

		slog.Info("assemble.capability",
			slog.String("name", cap.Name),
			slog.Float64("risk", cap.Risk),
			slog.String("source", cap.Source.String()))
	}

	if len(bp.Capabilities) == 0 {
		slog.Warn("assemble.no_capabilities")
	}

	var cb *cerebrum.Cerebrum

	motorBus := corp.MotorBus(sensory, nil, nil)

	cb = cerebrum.New(reactor, completer,
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(budget),
		cerebrum.WithTools(tools),
		cerebrum.WithCapabilities(bp.Capabilities),
		cerebrum.WithConfig(cfg),
	)

	slog.Info("assemble.complete",
		slog.String("model", bp.Model),
		slog.Int("capabilities", len(bp.Capabilities)),
		slog.Int("max_turns", budget.MaxTurns))

	return &Agent{
		Cerebrum: cb,
		Sensory:  sensory,
		Corpus:   corp,
	}
}

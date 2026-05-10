package arcade

import (
	"context"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/testkit/rehearsal"
	tangle "github.com/dpopsuev/tangle"
)

type ArcadeConfig struct {
	MaxTurns    int
	TurnTimeout time.Duration
	Embedder    cerebrum.Embedder
	ReflexStore cerebrum.ReflexStore
	Consolidator cerebrum.Consolidator
	Listener    cerebrum.ContextListener
}

type ArcadeOption func(*ArcadeConfig)

func WithMaxTurns(n int) ArcadeOption {
	return func(c *ArcadeConfig) { c.MaxTurns = n }
}

func WithEmbedder(e cerebrum.Embedder) ArcadeOption {
	return func(c *ArcadeConfig) { c.Embedder = e }
}

func WithReflexStore(s cerebrum.ReflexStore) ArcadeOption {
	return func(c *ArcadeConfig) { c.ReflexStore = s }
}

func WithConsolidator(cons cerebrum.Consolidator) ArcadeOption {
	return func(c *ArcadeConfig) { c.Consolidator = cons }
}

func WithListener(l cerebrum.ContextListener) ArcadeOption {
	return func(c *ArcadeConfig) { c.Listener = l }
}

func BuildArcadeAgent(scenario Scenario, completer tangle.Completer, opts ...ArcadeOption) *assemble.Agent {
	cfg := ArcadeConfig{
		MaxTurns:    20,
		TurnTimeout: 60 * time.Second,
	}
	for _, o := range opts {
		o(&cfg)
	}

	bp := assemble.Blueprint{
		Model:  "arcade",
		Organs: scenario.Adventure.Organs(),
		Budget: cerebrum.Budget{
			MaxTurns:    cfg.MaxTurns,
			TurnTimeout: cfg.TurnTimeout,
		},
	}

	observe := func() map[string]any {
		return map[string]any{"world": scenario.Adventure.Observe()}
	}

	cerebrumOpts := []cerebrum.Option{
		cerebrum.WithObserver(observe),
	}
	if cfg.Embedder != nil {
		cerebrumOpts = append(cerebrumOpts, cerebrum.WithEmbedder(cfg.Embedder))
	}
	if cfg.ReflexStore != nil {
		cerebrumOpts = append(cerebrumOpts, cerebrum.WithReflexStore(cfg.ReflexStore))
	}
	if cfg.Consolidator != nil {
		cerebrumOpts = append(cerebrumOpts, cerebrum.WithConsolidator(cfg.Consolidator))
	}
	if cfg.Listener != nil {
		cerebrumOpts = append(cerebrumOpts, cerebrum.WithContextListener(cfg.Listener))
	}

	return assemble.Assemble(bp, completer, cerebrumOpts...)
}

func NewFlywheel(embedder cerebrum.Embedder) (cerebrum.ReflexStore, cerebrum.Consolidator) {
	store := cerebrum.NewPipeStore()
	consolidator := &cerebrum.PipeConsolidator{
		Store:    store,
		Embedder: embedder,
	}
	return store, consolidator
}

func ThinkScenario(ctx context.Context, agent *assemble.Agent, scenario Scenario) (cerebrum.ThinkOutcome, error) {
	catalyst := reactivity.Catalyst{
		Need:    scenario.Need,
		Desired: scenario.Desired,
	}
	return agent.ThinkWith(ctx, catalyst)
}

func ArcadeReferee(scenario Scenario) *rehearsal.GameReferee {
	return &rehearsal.GameReferee{
		IsSolved: scenario.IsSolved,
		State:    scenario.Adventure.State,
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dpopsuev/tako/assemble"
	tangle "github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/providers"
	anyllm "github.com/mozilla-ai/any-llm-go/providers"
)

func agentCmd(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	blueprintPath := fs.String("blueprint", "", "path to Blueprint YAML")
	provider := fs.String("provider", "", "LLM provider (claude, vertex-ai, gemini, openrouter)")
	model := fs.String("model", "", "model name (default: claude-sonnet-4-6)")
	verbose := fs.Bool("v", false, "verbose output (debug level)")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: tako agent [--blueprint FILE] [--provider NAME] [--model NAME] 'task'")
	}
	task := fs.Arg(0)

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	var bp assemble.Blueprint
	if *blueprintPath != "" {
		cfg, err := assemble.LoadBlueprint(*blueprintPath)
		if err != nil {
			return err
		}
		bp = cfg.ToBlueprint()
	} else {
		bp = defaultBlueprint()
	}

	if *model != "" {
		bp.Model = *model
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	completer, err := newAgentCompleter(ctx, *provider, bp.Model)
	if err != nil {
		return fmt.Errorf("completer: %w", err)
	}

	if bp.ModelWatcher != "" {
		watcher, err := newAgentCompleter(ctx, *provider, bp.ModelWatcher)
		if err != nil {
			return fmt.Errorf("watcher completer: %w", err)
		}
		bp.Watcher = watcher
	}

	agent := assemble.Assemble(bp, completer)

	slog.Info("tako.agent.start", slog.String("task", task), slog.String("model", bp.Model))

	if err := agent.Think(ctx, task); err != nil {
		return fmt.Errorf("think: %w", err)
	}

	m := agent.Result()
	slog.Info("tako.agent.done",
		slog.Bool("sealed", m.Sealed()),
		slog.Float64("distance", m.Distance()),
		slog.Int("mass", m.TotalMass()))

	return nil
}

func defaultBlueprint() assemble.Blueprint {
	wd, _ := os.Getwd()
	cfg := assemble.BlueprintConfig{
		Model:        "claude-sonnet-4-6",
		Capabilities: []string{"code"},
		WorkDir:      wd,
		Budget: assemble.BudgetConfig{
			MaxTurns:    30,
			TurnTimeout: "30s",
		},
	}
	return cfg.ToBlueprint()
}

func newAgentCompleter(_ context.Context, providerName, model string) (tangle.Completer, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	var p anyllm.Provider
	var err error
	if providerName != "" {
		p, err = providers.NewProviderByName(providerName)
	} else {
		p, err = providers.NewProviderFromEnv("TAKO_PROVIDER")
	}
	if err != nil {
		return nil, err
	}

	slog.Info("tako.agent.provider", slog.String("model", model))

	return providers.NewCompleter(p, model, nil), nil
}

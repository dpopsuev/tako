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
)

const defaultModel = "claude-sonnet-4-6"

func resolveModel() string {
	if m := os.Getenv("TAKO_MODEL"); m != "" {
		return m
	}
	return defaultModel
}

func agentCmd(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	blueprintPath := fs.String("blueprint", "", "path to Blueprint YAML")
	provider := fs.String("provider", "", "LLM provider (vertex-ai, anthropic, openai, gemini, openrouter)")
	model := fs.String("model", "", "model name (env: TAKO_MODEL, default: "+defaultModel+")")
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

	if *blueprintPath == "" {
		if _, err := os.Stat(projectBlueprint()); err == nil {
			*blueprintPath = projectBlueprint()
			slog.Info("tako.blueprint.auto", slog.String("path", *blueprintPath))
		}
	}

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

	result, err := agent.Run(ctx, task)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}

	m := agent.Result()
	slog.Info("tako.agent.done",
		slog.Bool("sealed", m.Sealed()),
		slog.Float64("distance", m.Distance()),
		slog.Int("mass", m.TotalMass()))

	fmt.Println(result)

	return nil
}

func defaultBlueprint() assemble.Blueprint {
	wd, _ := os.Getwd()
	cfg := assemble.BlueprintConfig{
		Model:        resolveModel(),
		Capabilities: []string{"code"},
		WorkDir:      wd,
		Budget: assemble.BudgetConfig{
			MaxTurns:    30,
			TurnTimeout: "120s",
		},
	}
	return cfg.ToBlueprint()
}

func newAgentCompleter(_ context.Context, providerName, model string) (tangle.Completer, error) {
	resolved := resolveProvider(providerName)
	if resolved == "" {
		_, err := providers.NewProviderFromEnv("TAKO_PROVIDER")
		return nil, err
	}

	completer, err := providers.NewCompleterByName(resolved, model)
	if err != nil {
		return nil, err
	}

	slog.Info("tako.agent.provider", slog.String("provider", resolved), slog.String("model", model))

	return completer, nil
}

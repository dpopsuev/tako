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
	"github.com/dpopsuev/tangle/arsenal"
	"github.com/dpopsuev/tangle/providers"
)

func agentCmd(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	blueprintPath := fs.String("blueprint", "", "path to Blueprint YAML")
	verbose := fs.Bool("v", false, "verbose output (debug level)")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: tako agent --blueprint FILE 'task description'")
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	completer, err := newAgentCompleter(ctx, bp.Model)
	if err != nil {
		return fmt.Errorf("completer: %w", err)
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

	if m.Distance() > 0 {
		return fmt.Errorf("task not completed (distance=%.2f)", m.Distance())
	}
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

func newAgentCompleter(ctx context.Context, model string) (tangle.Completer, error) {
	region := os.Getenv("CLOUD_ML_REGION")
	project := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if region == "" || project == "" {
		return nil, fmt.Errorf("CLOUD_ML_REGION and ANTHROPIC_VERTEX_PROJECT_ID required")
	}

	ars, err := arsenal.NewArsenal("")
	if err != nil {
		return nil, err
	}

	if model == "" {
		model = "claude-sonnet-4-6"
	}
	resolved, err := ars.Pick(model, "vertex-ai")
	if err != nil {
		return nil, err
	}

	provider, err := providers.NewVertexProvider(ctx, region, project)
	if err != nil {
		return nil, err
	}
	return providers.NewCompleter(provider, resolved.Model, nil), nil
}

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/assemble"
	takoTUI "github.com/dpopsuev/tako/tui"
)

func tuiCmd(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	blueprintPath := fs.String("blueprint", "", "path to Blueprint YAML")
	provider := fs.String("provider", "", "LLM provider (vertex-ai, anthropic-api, etc.)")
	model := fs.String("model", "", "model name (env: TAKO_MODEL, default: "+defaultModel+")")
	_ = fs.Parse(args)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	if *blueprintPath == "" {
		if _, err := os.Stat(".tako/blueprint.yaml"); err == nil {
			*blueprintPath = ".tako/blueprint.yaml"
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

	completer, err := newAgentCompleter(nil, *provider, bp.Model)
	if err != nil {
		return fmt.Errorf("completer: %w", err)
	}

	if bp.ModelWatcher != "" {
		watcher, err := newAgentCompleter(nil, *provider, bp.ModelWatcher)
		if err != nil {
			return fmt.Errorf("watcher completer: %w", err)
		}
		bp.Watcher = watcher
	}

	adapter := &takoTUI.Adapter{}

	agent := assemble.Assemble(bp, completer,
		cerebrum.WithContextListener(adapter),
	)

	m := takoTUI.NewModel(agent, bp.Model) // agent satisfies tui.Runner
	p := tea.NewProgram(m, tea.WithAltScreen())
	adapter.Program = p

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

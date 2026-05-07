package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/assemble"
	takoTUI "github.com/dpopsuev/tako/tui"
)

func tuiCmd(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	blueprintPath := fs.String("blueprint", "", "path to Blueprint YAML")
	provider := fs.String("provider", "", "LLM provider (vertex-ai, anthropic-api, etc.)")
	model := fs.String("model", "", "model name (default: claude-sonnet-4-6)")
	_ = fs.Parse(args)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

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

	agent := assemble.Assemble(bp, completer)

	m := takoTUI.NewModel(agent, bp.Model)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)
	takoTUI.SubscribeMolecule(agent, p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

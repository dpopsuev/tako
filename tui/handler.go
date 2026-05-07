package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/widgets"
)

type Runner interface {
	Run(ctx context.Context, task string) (string, error)
}

func runAgentCmd(runner Runner, task string) tea.Cmd {
	return func() tea.Msg {
		_, err := runner.Run(context.Background(), task)
		if err != nil {
			return widgets.ErrorMsg{Err: err}
		}
		return nil
	}
}

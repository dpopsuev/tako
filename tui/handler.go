package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/tui/widgets"
)

func runAgentCmd(agent *assemble.Agent, task string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := agent.Run(ctx, task)
		if err != nil {
			return widgets.ErrorMsg{Err: err}
		}
		return nil
	}
}

package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type Runner interface {
	Run(ctx context.Context, task string) error
}

type agentStartedMsg struct{}

func runAgentCmd(runner Runner, task string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			runner.Run(context.Background(), task)
		}()
		return agentStartedMsg{}
	}
}

func waitForMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

package widgets

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
)

type StatusPanel struct {
	core.BasePanel
	model string
}

func NewStatusPanel(model string) *StatusPanel {
	return &StatusPanel{
		BasePanel: core.NewBasePanel("status", 1),
		model:     model,
	}
}

var _ core.Panel = (*StatusPanel)(nil)

func (p *StatusPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	return p, nil
}

func (p *StatusPanel) View(width int) string {
	return fmt.Sprintf(" tako · %s", p.model)
}

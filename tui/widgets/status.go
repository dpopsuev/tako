package widgets

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
)

type StatusPanel struct {
	core.BasePanel
	phase    string
	turn     int
	distance float64
	model    string
	sealed   bool
}

func NewStatusPanel(model string) *StatusPanel {
	return &StatusPanel{
		BasePanel: core.NewBasePanel("status", 1),
		model:     model,
		phase:     "idle",
	}
}

var _ core.Panel = (*StatusPanel)(nil)

func (p *StatusPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case PhaseChangeMsg:
		p.phase = msg.Phase
		p.turn = msg.Turn
	case AgentDoneMsg:
		p.sealed = msg.Sealed
		p.distance = msg.Distance
	}
	return p, nil
}

func (p *StatusPanel) View(width int) string {
	if p.sealed {
		return fmt.Sprintf(" done | %d turns | d=%.2f | %s", p.turn, p.distance, p.model)
	}
	return fmt.Sprintf(" %s | turn %d | d=%.2f | %s", p.phase, p.turn, p.distance, p.model)
}

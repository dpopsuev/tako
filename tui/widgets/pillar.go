package widgets

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
)

type PillarPanel struct {
	core.BasePanel
	height int
}

func NewPillarPanel(id string) *PillarPanel {
	return &PillarPanel{
		BasePanel: core.NewBasePanel(id, 0),
	}
}

var _ core.Panel = (*PillarPanel)(nil)

func (p *PillarPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	if rm, ok := msg.(layout.ResizeMsg); ok {
		p.height = rm.Height
	}
	return p, nil
}

func (p *PillarPanel) View(width int) string {
	h := p.height
	if h <= 0 {
		h = 1
	}
	pad := strings.Repeat(" ", width)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = pad
	}
	return strings.Join(lines, "\n")
}

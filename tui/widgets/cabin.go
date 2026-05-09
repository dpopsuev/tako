package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
)

type CabinCenter struct {
	core.BasePanel
	left   *PillarPanel
	right  *PillarPanel
	output *OutputPanel
	input  *InputPanel
	width  int
	height int
}

func NewCabinCenter(output *OutputPanel, input *InputPanel, left, right *PillarPanel) *CabinCenter {
	return &CabinCenter{
		BasePanel: core.NewBasePanel("cabin-center", 0),
		left:      left,
		right:     right,
		output:    output,
		input:     input,
	}
}

var _ core.Panel = (*CabinCenter)(nil)

func (c *CabinCenter) Children() []core.Panel {
	return []core.Panel{c.left, c.output, c.input, c.right}
}

func (c *CabinCenter) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	if rm, ok := msg.(layout.ResizeMsg); ok {
		c.width = rm.Width
		c.height = rm.Height

		pillarW := c.width / 8
		centerW := c.width - 2*pillarW
		inputH := 3
		outputH := c.height - inputH - 1
		if outputH < 3 {
			outputH = 3
		}

		c.left.Update(layout.ResizeMsg{Width: pillarW, Height: c.height})
		c.right.Update(layout.ResizeMsg{Width: pillarW, Height: c.height})
		c.output.Update(layout.ResizeMsg{Width: centerW, Height: outputH})
		c.input.Update(layout.ResizeMsg{Width: centerW, Height: inputH})
	}
	return c, nil
}

func (c *CabinCenter) View(width int) string {
	pillarW := width / 8
	centerW := width - 2*pillarW
	inputH := 3
	outputH := c.height - inputH - 1
	if outputH < 3 {
		outputH = 3
	}

	outputView := c.output.View(centerW)
	inputView := c.input.View(centerW)
	separator := strings.Repeat("─", centerW)

	centerCol := lipgloss.JoinVertical(lipgloss.Left,
		outputView,
		separator,
		inputView,
	)

	leftView := c.left.View(pillarW)
	rightView := c.right.View(pillarW)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftView,
		centerCol,
		rightView,
	)
}

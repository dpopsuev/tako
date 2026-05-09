package widgets

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		c.resizeChildren()
	}
	return c, nil
}

func (c *CabinCenter) resizeChildren() {
	pillarW := c.pillarWidth()
	centerW := c.centerWidth()
	inputH := 4
	outputH := c.height - inputH - 3
	if outputH < 1 {
		outputH = 1
	}

	c.left.Update(layout.ResizeMsg{Width: pillarW, Height: c.height - 2})
	c.right.Update(layout.ResizeMsg{Width: pillarW, Height: c.height - 2})
	c.output.Update(layout.ResizeMsg{Width: centerW, Height: outputH})
	c.input.Update(layout.ResizeMsg{Width: centerW - 2, Height: inputH})
}

func (c *CabinCenter) pillarWidth() int {
	w := c.width / 8
	if w < 2 {
		w = 2
	}
	return w
}

func (c *CabinCenter) centerWidth() int {
	return c.width - 2*c.pillarWidth() - 2
}

func (c *CabinCenter) View(width int) string {
	pillarW := c.pillarWidth()
	centerW := c.centerWidth()
	inputH := 4
	outputH := c.height - inputH - 3
	if outputH < 1 {
		outputH = 1
	}

	hrFull := strings.Repeat("─", c.width)

	outputContent := c.output.View(centerW)
	outputLines := strings.Split(outputContent, "\n")

	for len(outputLines) < outputH {
		outputLines = append([]string{padRight("", centerW)}, outputLines...)
	}
	if len(outputLines) > outputH {
		outputLines = outputLines[len(outputLines)-outputH:]
	}

	inputContent := c.input.View(centerW - 2)
	inputLines := strings.Split(inputContent, "\n")
	for len(inputLines) < inputH {
		inputLines = append(inputLines, padRight("", centerW-2))
	}

	leftPad := strings.Repeat(" ", pillarW)
	rightPad := strings.Repeat(" ", pillarW)
	separator := strings.Repeat("─", centerW)

	var sb strings.Builder

	sb.WriteString(hrFull)
	sb.WriteByte('\n')

	for _, line := range outputLines {
		sb.WriteString(leftPad)
		sb.WriteString("│")
		sb.WriteString(padRight(line, centerW))
		sb.WriteString("│")
		sb.WriteString(rightPad)
		sb.WriteByte('\n')
	}

	sb.WriteString(leftPad)
	sb.WriteString("│")
	sb.WriteString(separator)
	sb.WriteString("│")
	sb.WriteString(rightPad)
	sb.WriteByte('\n')

	for _, line := range inputLines {
		sb.WriteString(leftPad)
		sb.WriteString("│")
		sb.WriteString(" ")
		sb.WriteString(padRight(line, centerW-2))
		sb.WriteString(" ")
		sb.WriteString("│")
		sb.WriteString(rightPad)
		sb.WriteByte('\n')
	}

	sb.WriteString(hrFull)

	return sb.String()
}

func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

package widgets

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
)

var innerBorder = lipgloss.Border{
	Top: "━", Bottom: "━", Left: "┃", Right: "┃",
	TopLeft: "┏", TopRight: "┓", BottomLeft: "┗", BottomRight: "┛",
}

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
	}
	return c, nil
}

func (c *CabinCenter) View(width int) string {
	pillarW := width / 8
	if pillarW < 1 {
		pillarW = 1
	}

	outerBorderW := 2
	innerBorderW := 2
	centerW := width - 2*pillarW - outerBorderW - innerBorderW
	if centerW < 10 {
		centerW = 10
	}

	inputH := 4
	innerH := c.height - 2
	if innerH < inputH+1 {
		innerH = inputH + 1
	}
	outputH := innerH - inputH - 3
	if outputH < 1 {
		outputH = 1
	}

	c.output.Update(layout.ResizeMsg{Width: centerW, Height: outputH})
	c.input.Update(layout.ResizeMsg{Width: centerW - 2, Height: inputH})

	outputContent := c.output.View(centerW)
	outputLines := strings.Split(outputContent, "\n")
	for len(outputLines) < outputH {
		outputLines = append([]string{""}, outputLines...)
	}
	if len(outputLines) > outputH {
		outputLines = outputLines[len(outputLines)-outputH:]
	}

	inputContent := c.input.View(centerW - 2)
	inputLines := strings.Split(inputContent, "\n")
	for len(inputLines) < inputH {
		inputLines = append(inputLines, "")
	}

	var centerLines []string
	for _, line := range outputLines {
		centerLines = append(centerLines, padToWidth(line, centerW))
	}
	centerLines = append(centerLines, strings.Repeat("━", centerW))
	for _, line := range inputLines {
		centerLines = append(centerLines, " "+padToWidth(line, centerW-2)+" ")
	}

	centerContent := strings.Join(centerLines, "\n")

	innerBox := lipgloss.NewStyle().
		Border(innerBorder).
		Width(centerW).
		Render(centerContent)

	leftPad := strings.Repeat(" ", pillarW)
	rightPad := strings.Repeat(" ", pillarW)

	boxLines := strings.Split(innerBox, "\n")
	var rows []string
	for _, line := range boxLines {
		rows = append(rows, leftPad+line+rightPad)
	}

	return strings.Join(rows, "\n")
}

func padToWidth(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

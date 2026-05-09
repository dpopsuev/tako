package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
)

var outerFrame = lipgloss.Border{
	Top: "═", Bottom: "═", Left: "║", Right: "║",
	TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝",
}

var innerFrame = lipgloss.Border{
	Top: "━", Bottom: "━", Left: "┃", Right: "┃",
	TopLeft: "┏", TopRight: "┓", BottomLeft: "┗", BottomRight: "┛",
}

type CabinCenter struct {
	core.BasePanel
	output    *OutputPanel
	input     *InputPanel
	modelName string
	width     int
	height    int
	phase     string
	turn      int
	distance  float64
	tokensIn  int
	tokensOut int
	toolCalls int
}

func NewCabinCenter(output *OutputPanel, input *InputPanel, modelName string) *CabinCenter {
	return &CabinCenter{
		BasePanel: core.NewBasePanel("cabin-center", 0),
		output:    output,
		input:     input,
		modelName: modelName,
		phase:     "idle",
	}
}

var _ core.Panel = (*CabinCenter)(nil)

func (c *CabinCenter) Children() []core.Panel {
	return []core.Panel{c.output, c.input}
}

func (c *CabinCenter) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case layout.ResizeMsg:
		c.width = msg.Width
		c.height = msg.Height
	case PhaseChangeMsg:
		c.phase = msg.Phase
		c.turn = msg.Turn
	case TokenUpdateMsg:
		c.tokensIn += msg.TokensIn
		c.tokensOut += msg.TokensOut
		c.toolCalls += msg.ToolCalls
	case AgentDoneMsg:
		c.distance = msg.Distance
	}
	return c, nil
}

func (c *CabinCenter) View(width int) string {
	if c.height < 8 {
		return ""
	}

	outerBorderCols := 2
	pillarW := (width - outerBorderCols) / 8
	if pillarW < 1 {
		pillarW = 1
	}
	innerBorderCols := 2
	centerW := width - outerBorderCols - 2*pillarW - innerBorderCols
	if centerW < 10 {
		centerW = 10
	}

	outerH := c.height - 2
	innerBorderRows := 2
	inputH := 4
	separatorH := 1
	outputH := outerH - innerBorderRows - inputH - separatorH
	if outputH < 1 {
		outputH = 1
	}

	c.output.Update(layout.ResizeMsg{Width: centerW, Height: outputH})
	c.input.Update(layout.ResizeMsg{Width: centerW, Height: inputH})

	outputView := c.output.View(centerW)
	outputLines := strings.Split(outputView, "\n")
	for len(outputLines) < outputH {
		outputLines = append([]string{""}, outputLines...)
	}
	if len(outputLines) > outputH {
		outputLines = outputLines[len(outputLines)-outputH:]
	}

	inputView := c.input.View(centerW)
	inputLines := strings.Split(inputView, "\n")
	for len(inputLines) < inputH {
		inputLines = append(inputLines, "")
	}

	var contentLines []string
	for _, l := range outputLines {
		contentLines = append(contentLines, pad(l, centerW))
	}
	contentLines = append(contentLines, strings.Repeat("━", centerW))
	for _, l := range inputLines {
		contentLines = append(contentLines, pad(l, centerW))
	}

	innerContent := strings.Join(contentLines, "\n")
	innerBox := lipgloss.NewStyle().
		Border(innerFrame).
		Width(centerW).
		Render(innerContent)

	innerLines := strings.Split(innerBox, "\n")

	pillarPad := strings.Repeat(" ", pillarW)

	var bodyLines []string
	for _, row := range innerLines {
		bodyLines = append(bodyLines, pillarPad+row+pillarPad)
	}
	for len(bodyLines) < outerH {
		bodyLines = append(bodyLines, strings.Repeat(" ", width-outerBorderCols))
	}

	body := strings.Join(bodyLines, "\n")

	title := fmt.Sprintf("═══ tako · %s ", c.modelName)
	footer := fmt.Sprintf("═══ %s · t%d · d=%.2f ", c.phase, c.turn, c.distance)
	footerRight := fmt.Sprintf(" ↑%s ↓%s · %dt ═",
		formatTokens(c.tokensIn), formatTokens(c.tokensOut), c.toolCalls)

	outerStyle := lipgloss.NewStyle().
		Border(outerFrame).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Width(width - outerBorderCols)

	rendered := outerStyle.Render(body)

	lines := strings.Split(rendered, "\n")

	if len(lines) > 0 {
		topW := width - lipgloss.Width(title) - 2
		if topW < 0 {
			topW = 0
		}
		lines[0] = "╔" + title + strings.Repeat("═", topW) + "╗"
	}

	if len(lines) > 1 {
		botW := width - lipgloss.Width(footer) - lipgloss.Width(footerRight) - 2
		if botW < 0 {
			botW = 0
		}
		lines[len(lines)-1] = "╚" + footer + strings.Repeat("═", botW) + footerRight + "╝"
	}

	return strings.Join(lines, "\n")
}

func formatTokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 10000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%dk", n/1000)
	}
}

func pad(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

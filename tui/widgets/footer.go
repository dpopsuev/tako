package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
)

type FooterPanel struct {
	core.BasePanel
	phase     string
	turn      int
	distance  float64
	sealed    bool
	tokensIn  int
	tokensOut int
	toolCalls int
}

func NewFooterPanel() *FooterPanel {
	return &FooterPanel{
		BasePanel: core.NewBasePanel("footer", 1),
		phase:     "idle",
	}
}

var _ core.Panel = (*FooterPanel)(nil)

func (p *FooterPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case PhaseChangeMsg:
		p.phase = msg.Phase
		p.turn = msg.Turn
	case TokenUpdateMsg:
		p.tokensIn += msg.TokensIn
		p.tokensOut += msg.TokensOut
		p.toolCalls += msg.ToolCalls
	case AgentDoneMsg:
		p.sealed = msg.Sealed
		p.distance = msg.Distance
	}
	return p, nil
}

func (p *FooterPanel) View(width int) string {
	left := fmt.Sprintf(" %s · t%d · d=%.2f ", p.phase, p.turn, p.distance)
	right := fmt.Sprintf(" ↑%s ↓%s · %dt ",
		formatTokens(p.tokensIn), formatTokens(p.tokensOut), p.toolCalls)

	fill := width - len(left) - len(right) - 5
	if fill < 0 {
		fill = 0
	}

	return "╚═══" + left + strings.Repeat("═", fill) + right + "═╝"
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

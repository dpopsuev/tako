package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/widgets"
)

type Adapter struct {
	Program *tea.Program
}

func (a *Adapter) OnContext(phase string, turn int, distance float64) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.PhaseChangeMsg{
		Phase: phase,
		Turn:  turn + 1,
	})
}

func (a *Adapter) OnTokenUpdate(tokensIn, tokensOut, toolCalls int) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.TokenUpdateMsg{
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		ToolCalls: toolCalls,
	})
}

func (a *Adapter) OnToolCall(name string, input []byte) {
	if a.Program == nil {
		return
	}
	s := string(input)
	if len(s) > 100 {
		s = s[:100] + "..."
	}
	a.Program.Send(widgets.ToolCallStartMsg{
		Name:  name,
		Input: s,
	})
}

func (a *Adapter) OnToolResult(name string, result []byte, _ time.Duration) {
	if a.Program == nil {
		return
	}
	s := string(result)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	a.Program.Send(widgets.ToolCallResultMsg{
		Name:   name,
		Result: s,
	})
}

func (a *Adapter) OnResponse(text string) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.AppendOutputMsg{Line: text})
}

func (a *Adapter) OnSealed(_ string, distance float64, turns int) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.AgentDoneMsg{
		Sealed:   true,
		Distance: distance,
		Turns:    turns,
	})
}

func (a *Adapter) OnError(turn int, err error) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.ErrorMsg{Err: err})
}

func (a *Adapter) OnToken(token string) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.StreamTokenMsg(token))
}

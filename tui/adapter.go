package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/tui/widgets"
)

type Adapter struct {
	Program *tea.Program
}

var _ cerebrum.ContextListener = (*Adapter)(nil)

func (a *Adapter) OnContext(ctx cerebrum.Context, turn int) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.PhaseChangeMsg{
		Phase: ctx.Phase.String(),
		Turn:  turn,
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

func (a *Adapter) OnSealed(_ string, distance float64, turns int, result string) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.AgentDoneMsg{
		Sealed:   true,
		Distance: distance,
		Turns:    turns,
		Result:   result,
	})
}

func (a *Adapter) OnError(turn int, err error) {
	if a.Program == nil {
		return
	}
	a.Program.Send(widgets.ErrorMsg{Err: err})
}

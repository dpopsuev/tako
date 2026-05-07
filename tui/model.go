package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
	"github.com/dpopsuev/tako/tui/widgets"
)

type Model struct {
	runner  Runner
	output  *widgets.OutputPanel
	input   *widgets.InputPanel
	status  *widgets.StatusPanel
	engine  *layout.LayoutEngine
	focus   *core.FocusManager
	width   int
	height  int
	running bool
}

func NewModel(runner Runner, modelName string) Model {
	output := widgets.NewOutputPanel()
	input := widgets.NewInputPanel()
	status := widgets.NewStatusPanel(modelName)

	fm := core.NewFocusManager(input, output)
	engine := layout.NewLayoutEngine(fm, plainBorder{})

	engine.Register(layout.PanelSlot{
		Panel:  status,
		Weight: 0,
		Border: layout.BorderNone,
	})
	engine.Register(layout.PanelSlot{
		Panel:     output,
		Weight:    1,
		MinHeight: 5,
		Focusable: true,
		Border:    layout.BorderNone,
	})
	engine.Register(layout.PanelSlot{
		Panel:     input,
		Weight:    0,
		Focusable: true,
		Border:    layout.BorderNone,
	})

	return Model{
		runner: runner,
		output: output,
		input:  input,
		status: status,
		engine: engine,
		focus:  fm,
	}
}

type plainBorder struct{}

func (plainBorder) RenderWithDepth(content string, _ int, _ int) string { return content }
func (plainBorder) RenderBorderOnly(content string, _ bool, _ int) string { return content }
func (plainBorder) FocusDepths(count, _ int) []int {
	d := make([]int, count)
	for i := range d { d[i] = 1 }
	return d
}

func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("tako")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.engine.Resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focus.Cycle()
			return m, nil
		}

	case widgets.SubmitMsg:
		if m.running {
			return m, nil
		}
		m.running = true
		m.output.Update(widgets.AppendOutputMsg{Line: "> " + msg.Text})
		m.output.Update(widgets.SetOverlayMsg{Text: "thinking..."})
		return m, runAgentCmd(m.runner, msg.Text)

	case agentStartedMsg:
		return m, nil

	case widgets.AgentDoneMsg:
		m.running = false
		m.output.Update(widgets.SetOverlayMsg{Text: ""})
		m.output.Update(widgets.AppendOutputMsg{
			Line: fmt.Sprintf("\n--- done: %d turns, d=%.2f ---",
				msg.Turns, msg.Distance),
		})
		if msg.Result != "" {
			m.output.Update(widgets.AppendOutputMsg{Line: "\n" + msg.Result})
		}
		m.status.Update(msg)
		return m, nil

	case widgets.ErrorMsg:
		m.running = false
		m.output.Update(widgets.SetOverlayMsg{Text: ""})
		m.output.Update(widgets.AppendOutputMsg{Line: "ERROR: " + msg.Err.Error()})
		return m, nil

	case widgets.PhaseChangeMsg:
		m.output.Update(msg)
		m.status.Update(msg)
		return m, nil

	case widgets.ToolCallStartMsg:
		m.output.Update(msg)
		return m, nil

	case widgets.ToolCallResultMsg:
		m.output.Update(msg)
		return m, nil

	case widgets.AppendOutputMsg:
		m.output.Update(msg)
		return m, nil
	}

	active := m.focus.Active()
	if active != nil {
		updated, cmd := active.Update(msg)
		if p, ok := updated.(*widgets.InputPanel); ok {
			m.input = p
		}
		if p, ok := updated.(*widgets.OutputPanel); ok {
			m.output = p
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "initializing..."
	}
	return m.engine.Render()
}

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
	cabin   *widgets.CabinCenter
	overlay *widgets.OverlayContainer
	engine  *layout.LayoutEngine
	focus   *core.FocusManager
	width   int
	height  int
	running bool
}

func NewModel(runner Runner, modelName string) Model {
	output := widgets.NewOutputPanel()
	input := widgets.NewInputPanel()
	cabin := widgets.NewCabinCenter(output, input, modelName)

	fm := core.NewFocusManager(input, output)
	engine := layout.NewLayoutEngine(fm, plainBorder{})

	engine.Register(layout.PanelSlot{
		Panel:     cabin,
		Weight:    1,
		MinHeight: 10,
		Border:    layout.BorderNone,
	})

	return Model{
		runner:  runner,
		output:  output,
		input:   input,
		cabin:   cabin,
		overlay: widgets.NewOverlayContainer(),
		engine:  engine,
		focus:   fm,
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

	case widgets.ShowOverlayMsg:
		m.overlay.Show(msg.Panel)
		return m, nil

	case widgets.HideOverlayMsg:
		m.overlay.Hide()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.overlay.Active() {
			if msg.String() == "esc" {
				m.overlay.Hide()
				return m, nil
			}
			cmd := m.overlay.Update(msg)
			return m, cmd
		}
		switch msg.String() {
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
		m.cabin.Update(msg)
		return m, nil

	case widgets.ErrorMsg:
		m.running = false
		m.output.Update(widgets.SetOverlayMsg{Text: ""})
		m.output.Update(widgets.AppendOutputMsg{Line: "ERROR: " + msg.Err.Error()})
		return m, nil

	case widgets.PhaseChangeMsg:
		m.output.Update(msg)
		m.cabin.Update(msg)
		return m, nil

	case widgets.TokenUpdateMsg:
		m.cabin.Update(msg)
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
	base := m.engine.Render()
	if m.overlay.Active() {
		return m.overlay.Render(base, m.width, m.height)
	}
	return base
}

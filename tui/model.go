package tui

import (
	"fmt"
	"strings"

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

	return Model{
		runner: runner,
		output: output,
		input:  input,
		status: status,
		focus:  fm,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("tako")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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

	outputHeight := m.height - 6
	if outputHeight < 3 {
		outputHeight = 3
	}

	p, _ := m.output.Update(layout.ResizeMsg{Width: m.width, Height: outputHeight})
	m.output = p.(*widgets.OutputPanel)

	var b strings.Builder
	b.WriteString(m.output.View(m.width))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")
	b.WriteString(m.status.View(m.width))
	b.WriteString("\n")
	b.WriteString(m.input.View(m.width))
	return b.String()
}

package widgets

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dpopsuev/tako/tui/core"
)

type SubmitMsg struct{ Text string }

type InputPanel struct {
	core.BasePanel
	ta textarea.Model
}

func NewInputPanel() *InputPanel {
	ta := textarea.New()
	ta.Placeholder = "Type a task..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Prompt = ""
	ta.FocusedStyle.Prompt = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Prompt = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.Focus()
	return &InputPanel{
		BasePanel: core.NewBasePanel("input", 3),
		ta:        ta,
	}
}

var _ core.Panel = (*InputPanel)(nil)

func (p *InputPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && p.Focused() {
		switch km.Type {
		case tea.KeyEnter:
			text := p.ta.Value()
			if text == "" {
				return p, nil
			}
			p.ta.Reset()
			return p, func() tea.Msg { return SubmitMsg{Text: text} }
		}
	}

	if p.Focused() {
		var cmd tea.Cmd
		p.ta, cmd = p.ta.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *InputPanel) View(width int) string {
	p.ta.SetWidth(width)
	return p.ta.View()
}

func (p *InputPanel) SetFocus(focused bool) {
	p.BasePanel.SetFocus(focused)
	if focused {
		p.ta.Focus()
	} else {
		p.ta.Blur()
	}
}

package widgets

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
	"github.com/dpopsuev/tako/tui/layout"
)

type OutputPanel struct {
	core.BasePanel
	vp        viewport.Model
	vpReady   bool
	lines     []string
	overlay   string
	dirty     bool
	streamBuf strings.Builder
}

func NewOutputPanel() *OutputPanel {
	return &OutputPanel{
		BasePanel: core.NewBasePanel("output", 0),
		dirty:     true,
	}
}

var _ core.Panel = (*OutputPanel)(nil)

func (p *OutputPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case AppendOutputMsg:
		p.lines = append(p.lines, msg.Line)
		p.dirty = true
	case StreamTokenMsg:
		p.streamBuf.WriteString(string(msg))
		p.dirty = true
	case FlushStreamMsg:
		if p.streamBuf.Len() > 0 {
			if len(p.lines) == 0 {
				p.lines = append(p.lines, "")
			}
			p.lines[len(p.lines)-1] += p.streamBuf.String()
			p.streamBuf.Reset()
			p.dirty = true
		}
	case ClearOutputMsg:
		p.lines = nil
		p.dirty = true
	case SetOverlayMsg:
		p.overlay = msg.Text
		p.dirty = true
	case ToolCallStartMsg:
		p.lines = append(p.lines, "["+msg.Name+"] "+msg.Input)
		p.dirty = true
	case ToolCallResultMsg:
		p.lines = append(p.lines, "["+msg.Name+"] "+truncateResult(msg.Result, 200))
		p.dirty = true
	case PhaseChangeMsg:
		p.overlay = msg.Phase
	case layout.ResizeMsg:
		if !p.vpReady {
			p.vp = viewport.New(msg.Width, msg.Height)
			p.vpReady = true
		} else {
			p.vp.Width = msg.Width
			p.vp.Height = msg.Height
		}
		p.dirty = true
	default:
		if !p.Focused() || !p.vpReady {
			return p, nil
		}
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *OutputPanel) View(width int) string {
	content := strings.Join(p.lines, "\n")
	if p.streamBuf.Len() > 0 {
		content += p.streamBuf.String()
	}
	if p.overlay != "" {
		content += "\n" + p.overlay
	}
	if p.vpReady {
		contentLines := strings.Count(content, "\n") + 1
		if contentLines < p.vp.Height {
			content = strings.Repeat("\n", p.vp.Height-contentLines) + content
		}
		if p.dirty {
			p.vp.SetContent(content)
			p.vp.GotoBottom()
			p.dirty = false
		}
		return p.vp.View()
	}
	return content
}

func truncateResult(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

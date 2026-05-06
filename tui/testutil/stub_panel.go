// stub_panel.go — Observable StubPanel for view transition tests.
//
// Records Update() calls and Focus changes for behavioral assertion.
// Richer than the minimal stubPanel in tui/views/views_test.go.
//
// GOL-188, TSK-1199
package testutil

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dpopsuev/tako/tui/core"
)

// StubPanel implements core.Panel with observable state.
type StubPanel struct {
	core.BasePanel
	Content string    // returned by View()
	Updates []tea.Msg // recorded Update() calls
}

var _ core.Panel = (*StubPanel)(nil)

// NewStubPanel creates a stub panel with the given ID and content.
func NewStubPanel(id, content string) *StubPanel {
	return &StubPanel{
		BasePanel: core.NewBasePanel(id, 1),
		Content:   content,
	}
}

func (p *StubPanel) Update(msg tea.Msg) (core.Panel, tea.Cmd) {
	p.Updates = append(p.Updates, msg)
	return p, nil
}

func (p *StubPanel) View(_ int) string {
	return p.Content
}

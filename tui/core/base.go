// base.go — BasePanel provides default Panel implementations.
// Embed in concrete panels to avoid boilerplate.
package core

import tea "github.com/charmbracelet/bubbletea"

// BasePanel provides default implementations for the Panel interface.
type BasePanel struct {
	id        string
	focused   bool
	collapsed bool
	height    int
}

// NewBasePanel creates a base panel with the given ID and height.
func NewBasePanel(id string, height int) BasePanel {
	return BasePanel{id: id, height: height}
}

func (b *BasePanel) ID() string          { return b.id }
func (b *BasePanel) Focused() bool       { return b.focused }
func (b *BasePanel) SetFocus(f bool)     { b.focused = f }
func (b *BasePanel) Children() []Panel   { return nil }
func (b *BasePanel) Height() int         { return b.height }
func (b *BasePanel) Collapsible() bool   { return false }
func (b *BasePanel) Collapsed() bool     { return b.collapsed }
func (b *BasePanel) Toggle()             { b.collapsed = !b.collapsed }
func (b *BasePanel) SetCollapsed(v bool) { b.collapsed = v }

func (b *BasePanel) Update(_ tea.Msg) (Panel, tea.Cmd) { return nil, nil }
func (b *BasePanel) View(_ int) string                 { return "" }

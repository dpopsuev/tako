// panel.go — recursive Panel interface for composable TUI.
// Panels nest panels (Composite pattern). Focus determines key behavior.
// Designed to support future splits, multi-agent, and dynamic resizing.
package core

import tea "github.com/charmbracelet/bubbletea"

// SplitDir is the direction of a split.
type SplitDir int

const (
	DirVertical   SplitDir = iota // top/bottom
	DirHorizontal                 // left/right
)

// Panel is the recursive composable TUI element.
// Every visual element — output, input, dashboard, envelope, widget — is a Panel.
type Panel interface {
	ID() string
	Focused() bool
	SetFocus(bool)
	Children() []Panel
	Height() int // requested height in lines (0 = flex, fills remaining)
	Collapsible() bool
	Collapsed() bool
	Toggle()
	Update(msg tea.Msg) (Panel, tea.Cmd)
	View(width int) string
}

// FocusManager routes input to the active panel.
// Tab cycles focus. Ctrl+W j/k moves focus up/down.
// Dive/Climb navigate into/out of child panels.
type FocusManager struct {
	panels []Panel
	active int
	stack  []focusFrame // focus stack for dive/climb navigation
}

// focusFrame stores state for returning from a dive.
type focusFrame struct {
	panels []Panel
	active int
}

// NewFocusManager creates a focus manager with the given panels.
// First panel receives initial focus.
func NewFocusManager(panels ...Panel) *FocusManager {
	fm := &FocusManager{panels: panels}
	if len(panels) > 0 {
		panels[0].SetFocus(true)
	}
	return fm
}

// Active returns the currently focused panel.
func (f *FocusManager) Active() Panel {
	if f.active < len(f.panels) {
		return f.panels[f.active]
	}
	return nil
}

// ActiveIndex returns the focused panel index.
func (f *FocusManager) ActiveIndex() int {
	return f.active
}

// Cycle moves focus to the next panel (Tab).
func (f *FocusManager) Cycle() {
	if len(f.panels) == 0 {
		return
	}
	f.panels[f.active].SetFocus(false)
	f.active = (f.active + 1) % len(f.panels)
	f.panels[f.active].SetFocus(true)
}

// FocusUp moves focus to the previous panel (Ctrl+W k).
func (f *FocusManager) FocusUp() {
	if len(f.panels) == 0 {
		return
	}
	f.panels[f.active].SetFocus(false)
	f.active--
	if f.active < 0 {
		f.active = len(f.panels) - 1
	}
	f.panels[f.active].SetFocus(true)
}

// FocusDown moves focus to the next panel (Ctrl+W j).
func (f *FocusManager) FocusDown() {
	f.Cycle()
}

// FocusPanel sets focus to a specific panel by index.
func (f *FocusManager) FocusPanel(idx int) {
	if idx < 0 || idx >= len(f.panels) {
		return
	}
	f.panels[f.active].SetFocus(false)
	f.active = idx
	f.panels[f.active].SetFocus(true)
}

// SetPanels replaces the panel list, preserving focus by panel ID.
// Called by LayoutEngine when visible panels change.
func (f *FocusManager) SetPanels(panels []Panel) {
	currentID := ""
	if f.active < len(f.panels) {
		currentID = f.panels[f.active].ID()
		f.panels[f.active].SetFocus(false)
	}
	f.panels = panels
	f.active = 0
	for i, p := range panels {
		if p.ID() == currentID {
			f.active = i
			break
		}
	}
	if len(panels) > 0 {
		panels[f.active].SetFocus(true)
	}
}

// Panels returns all managed panels.
func (f *FocusManager) Panels() []Panel {
	return f.panels
}

// Count returns the number of panels.
func (f *FocusManager) Count() int {
	return len(f.panels)
}

// Dive enters the active panel's children. Pushes current state onto stack.
func (f *FocusManager) Dive() bool {
	if f.active >= len(f.panels) {
		return false
	}
	children := f.panels[f.active].Children()
	if len(children) == 0 {
		return false
	}
	f.stack = append(f.stack, focusFrame{panels: f.panels, active: f.active})
	f.panels[f.active].SetFocus(false)
	f.panels = children
	f.active = 0
	f.panels[0].SetFocus(true)
	return true
}

// Climb returns to the parent panel level. Pops the stack.
func (f *FocusManager) Climb() bool {
	if len(f.stack) == 0 {
		return false
	}
	f.panels[f.active].SetFocus(false)
	frame := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	f.panels = frame.panels
	f.active = frame.active
	f.panels[f.active].SetFocus(true)
	return true
}

// Depth returns the current dive depth (0 = top level).
func (f *FocusManager) Depth() int {
	return len(f.stack)
}

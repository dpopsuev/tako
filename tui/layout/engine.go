// engine.go — declarative panel composition, separate from panel logic.
// Panels register with rules. Engine computes visibility, allocates height,
// renders borders, composes final view. model.go View() becomes one call.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dpopsuev/tako/tui/core"
)

// ResizeMsg is sent to panels when the layout engine resizes them.
type ResizeMsg struct{ Width, Height int }

// BorderRenderer abstracts border rendering so the engine does not
// depend on the parent tui package (avoiding circular imports).
type BorderRenderer interface {
	RenderWithDepth(content string, depth, width int) string
	RenderBorderOnly(content string, focused bool, width int) string
	FocusDepths(count, focusedIdx int) []int
}

// BorderMode controls panel border rendering.
type BorderMode int

const (
	BorderFocusDepth BorderMode = iota // focused=red, unfocused=dim
	BorderOnly                         // border without content dimming (output panel)
	BorderNone                         // no border
)

// PanelSlot registers a panel with visibility and layout rules.
type PanelSlot struct {
	Panel     core.Panel
	Visible   func() bool // nil = always visible
	Weight    int         // 0 = fixed height (uses Panel.Height()), >0 = flex
	MinHeight int         // minimum height for flex panels
	Border    BorderMode
	Focusable bool          // included in focus cycling
	Direction core.SplitDir // DirVertical (default, 0) or DirHorizontal (1)
	Group     string        // panels with same non-empty Group render side-by-side
}

// LayoutEngine computes panel positions and renders the final view.
type LayoutEngine struct {
	slots    []PanelSlot
	focus    *core.FocusManager
	renderer BorderRenderer
	width    int
	height   int
}

// NewLayoutEngine creates an engine with the given focus manager and border renderer.
func NewLayoutEngine(fm *core.FocusManager, br BorderRenderer) *LayoutEngine {
	return &LayoutEngine{focus: fm, renderer: br}
}

// Register adds a panel slot to the layout.
func (e *LayoutEngine) Register(slot PanelSlot) {
	e.slots = append(e.slots, slot)
}

// Resize updates terminal dimensions.
func (e *LayoutEngine) Resize(width, height int) {
	e.width = width
	e.height = height
}

// SetMinHeight updates the MinHeight of the slot containing the given panel.
func (e *LayoutEngine) SetMinHeight(panel core.Panel, minHeight int) {
	for i := range e.slots {
		if e.slots[i].Panel.ID() == panel.ID() {
			e.slots[i].MinHeight = minHeight
			return
		}
	}
}

// VisibleSlots returns slots where Visible() == true (or Visible is nil).
func (e *LayoutEngine) VisibleSlots() []PanelSlot {
	var out []PanelSlot
	for _, s := range e.slots {
		if s.Visible == nil || s.Visible() {
			out = append(out, s)
		}
	}
	return out
}

// FocusablePanels returns visible + focusable panels for FocusManager.
func (e *LayoutEngine) FocusablePanels() []core.Panel {
	var out []core.Panel
	for _, s := range e.VisibleSlots() {
		if s.Focusable {
			out = append(out, s.Panel)
		}
	}
	return out
}

// ComputeHeights distributes available height among visible panels.
func (e *LayoutEngine) ComputeHeights() map[string]int {
	visible := e.VisibleSlots()
	heights := make(map[string]int, len(visible))

	fixedTotal := 0
	flexTotal := 0
	for _, s := range visible {
		borderH := 0
		if s.Border != BorderNone {
			borderH = 2
		}
		if s.Weight == 0 {
			h := s.Panel.Height()
			if h == 0 {
				h = 1
			}
			heights[s.Panel.ID()] = h
			fixedTotal += h + borderH
		} else {
			flexTotal += s.Weight
		}
	}

	// Add newlines between panels.
	if len(visible) > 1 {
		fixedTotal += len(visible) - 1
	}

	remaining := e.height - fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	for _, s := range visible {
		if s.Weight > 0 {
			h := remaining
			if flexTotal > 0 {
				h = remaining * s.Weight / flexTotal
			}
			if h < s.MinHeight {
				h = s.MinHeight
			}
			heights[s.Panel.ID()] = h
		}
	}

	return heights
}

// visibleSlot pairs a PanelSlot with its focus index (or -1 if not focusable).
type visibleSlot struct {
	PanelSlot
	focusIdx int
}

// slotGroup is a run of consecutive visible slots that share the same Group.
type slotGroup struct {
	group string
	slots []visibleSlot
}

// renderHorizontalGroup renders a set of horizontal slots side-by-side.
// Width is distributed proportionally by Weight (same logic as ComputeHeights).
func (e *LayoutEngine) renderHorizontalGroup(slots []visibleSlot, totalWidth, height int, depths []int) string {
	n := len(slots)
	if n == 0 {
		return ""
	}

	// Distribute width proportionally by Weight.
	totalWeight := 0
	for _, s := range slots {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	widths := make([]int, n)
	allocated := 0
	for i, s := range slots {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		if i == n-1 {
			widths[i] = totalWidth - allocated // last panel gets remainder
		} else {
			widths[i] = totalWidth * w / totalWeight
		}
		allocated += widths[i]
	}

	// Render each panel with its allocated width and the group height.
	var rendered []string
	for i, vs := range slots {
		panelWidth := widths[i]
		innerW := panelWidth - 2
		if innerW < 4 {
			innerW = 4
		}

		// Resize flex panels.
		if vs.Weight > 0 {
			vs.Panel.Update(ResizeMsg{Width: innerW, Height: height})
		}

		content := vs.Panel.View(innerW)

		switch vs.Border {
		case BorderFocusDepth:
			depth := 1
			if vs.Focusable && vs.focusIdx >= 0 && vs.focusIdx < len(depths) {
				depth = depths[vs.focusIdx]
			}
			rendered = append(rendered, e.renderer.RenderWithDepth(content, depth, panelWidth))
		case BorderOnly:
			focused := false
			if vs.Focusable && vs.focusIdx >= 0 && vs.focusIdx < len(depths) {
				focused = depths[vs.focusIdx] == 0
			}
			rendered = append(rendered, e.renderer.RenderBorderOnly(content, focused, panelWidth))
		case BorderNone:
			rendered = append(rendered, content)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// Render produces the full TUI view string.
func (e *LayoutEngine) Render() string { //nolint:gocyclo // layout composition with horizontal/vertical splits
	visible := e.VisibleSlots()
	if len(visible) == 0 {
		return ""
	}

	// Sync FocusManager with visible focusable panels.
	e.focus.SetPanels(e.FocusablePanels())

	innerWidth := e.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}
	heights := e.ComputeHeights()
	depths := e.renderer.FocusDepths(e.focus.Count(), e.focus.ActiveIndex())

	// Build visibleSlots with focus indices.
	focusIdx := 0
	vSlots := make([]visibleSlot, 0, len(visible))
	for _, slot := range visible {
		vs := visibleSlot{PanelSlot: slot, focusIdx: -1}
		if slot.Focusable {
			vs.focusIdx = focusIdx
			focusIdx++
		}
		vSlots = append(vSlots, vs)
	}

	// Group consecutive slots with same non-empty Group.
	var groups []slotGroup
	for _, vs := range vSlots {
		if vs.Group != "" && len(groups) > 0 && groups[len(groups)-1].group == vs.Group {
			groups[len(groups)-1].slots = append(groups[len(groups)-1].slots, vs)
		} else {
			groups = append(groups, slotGroup{group: vs.Group, slots: []visibleSlot{vs}})
		}
	}

	var sb strings.Builder
	for i, g := range groups {
		if i > 0 {
			sb.WriteByte('\n')
		}

		// Check if this is a horizontal group (non-empty group, all DirHorizontal).
		isHorizontal := g.group != "" && len(g.slots) > 1
		if isHorizontal {
			for _, vs := range g.slots {
				if vs.Direction != core.DirHorizontal {
					isHorizontal = false
					break
				}
			}
		}

		if isHorizontal { //nolint:nestif // layout computation requires branching on orientation
			// Pick the height for this group from the first slot.
			h := heights[g.slots[0].Panel.ID()]
			sb.WriteString(e.renderHorizontalGroup(g.slots, e.width, h, depths))
		} else {
			// Render each slot vertically (original logic).
			for j, vs := range g.slots {
				if j > 0 {
					sb.WriteByte('\n')
				}

				// Resize flex panels.
				if vs.Weight > 0 {
					vs.Panel.Update(ResizeMsg{Width: innerWidth, Height: heights[vs.Panel.ID()]})
				}

				content := vs.Panel.View(innerWidth)

				switch vs.Border {
				case BorderFocusDepth:
					depth := 1
					if vs.Focusable && vs.focusIdx >= 0 && vs.focusIdx < len(depths) {
						depth = depths[vs.focusIdx]
					}
					sb.WriteString(e.renderer.RenderWithDepth(content, depth, e.width))
				case BorderOnly:
					focused := false
					if vs.Focusable && vs.focusIdx >= 0 && vs.focusIdx < len(depths) {
						focused = depths[vs.focusIdx] == 0
					}
					sb.WriteString(e.renderer.RenderBorderOnly(content, focused, e.width))
				case BorderNone:
					sb.WriteString(content)
				}
			}
		}
	}

	return sb.String()
}

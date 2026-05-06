// bridge.go — translates core.Slots from Views into PanelSlots for LayoutEngine.
//
// Column semantics:
//   - ColumnFull: vertical stacking (no group)
//   - ColumnMain + ColumnSide: same horizontal group, rendered side-by-side
//
// GOL-181
package layout

import "github.com/dpopsuev/tako/tui/core"

// columnGroup is the layout group name for 2-column panels.
const columnGroup = "columns"

// SlotsToLayout converts View slots into PanelSlots for the LayoutEngine.
// ColumnMain and ColumnSide panels are grouped horizontally.
// ColumnFull panels stack vertically.
func SlotsToLayout(slots core.Slots) []PanelSlot {
	result := make([]PanelSlot, 0, len(slots))

	for _, s := range slots {
		ps := PanelSlot{
			Panel:     s.Panel,
			Visible:   s.Visible,
			Weight:    s.Weight,
			MinHeight: s.MinHeight,
			Border:    mapBorder(s.Border),
			Focusable: s.Focusable,
		}

		if s.Column == core.ColumnMain || s.Column == core.ColumnSide {
			ps.Group = columnGroup
			ps.Direction = core.DirHorizontal
		}

		result = append(result, ps)
	}

	return result
}

// mapBorder converts core.SlotBorder to layout.BorderMode.
func mapBorder(b core.SlotBorder) BorderMode {
	switch b {
	case core.SlotBorderOnly:
		return BorderOnly
	case core.SlotBorderNone:
		return BorderNone
	default:
		return BorderFocusDepth
	}
}

// SetSlots replaces all registered slots atomically.
// Used during ViewMode switches to rebuild the layout.
func (e *LayoutEngine) SetSlots(slots []PanelSlot) {
	e.slots = slots
}

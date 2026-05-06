// slot.go — layout descriptor returned by Views.
//
// A Slot pairs a Panel with layout hints: weight, column placement,
// visibility. The LayoutEngine translates Slots into PanelSlots for
// rendering. This keeps Views decoupled from layout internals.
//
// GOL-181
package core

// Column identifies which column a panel belongs to in a 2-column layout.
type Column int

const (
	ColumnFull Column = iota // spans full width (default, single-column)
	ColumnMain               // left/main area in 2-column layout
	ColumnSide               // right/sidebar in 2-column layout
)

// SlotBorder controls how a panel's border is rendered.
// Parallel to layout.BorderMode — defined here to avoid core → layout import.
type SlotBorder int

const (
	SlotBorderDefault SlotBorder = iota // → BorderFocusDepth (focused=red, unfocused=dim)
	SlotBorderOnly                      // → BorderOnly (border without content dimming)
	SlotBorderNone                      // → BorderNone (no border)
)

// Slot describes a Panel's position and behavior within a View.
type Slot struct {
	Panel     Panel
	Weight    int         // 0 = fixed height (Panel.Height()), >0 = flex
	MinHeight int         // minimum height for flex panels
	Column    Column      // which column this slot belongs to
	Border    SlotBorder  // border rendering mode
	Visible   func() bool // nil = always visible
	Focusable bool        // included in focus cycling
}

// Slots is a named slice of Slot.
type Slots []Slot

// Panels extracts the Panel references from all slots.
func (s Slots) Panels() Panels {
	out := make(Panels, len(s))
	for i, slot := range s {
		out[i] = slot.Panel
	}
	return out
}

// Panels is a named slice of Panel.
type Panels []Panel

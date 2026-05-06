package layout

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dpopsuev/tako/tui/core"
)

type stubPanel struct{ id string }

func (p *stubPanel) ID() string                             { return p.id }
func (p *stubPanel) Focused() bool                          { return false }
func (p *stubPanel) SetFocus(_ bool)                        {}
func (p *stubPanel) Children() []core.Panel                 { return nil }
func (p *stubPanel) Height() int                            { return 1 }
func (p *stubPanel) Collapsible() bool                      { return false }
func (p *stubPanel) Collapsed() bool                        { return false }
func (p *stubPanel) Toggle()                                {}
func (p *stubPanel) Update(_ tea.Msg) (core.Panel, tea.Cmd) { return p, nil }
func (p *stubPanel) View(_ int) string                      { return p.id }

func sp(id string) *stubPanel { return &stubPanel{id: id} }

func TestSlotsToLayout_FullColumn(t *testing.T) {
	slots := core.Slots{
		{Panel: sp("output"), Weight: 1, Focusable: true},
		{Panel: sp("input"), Focusable: true},
	}

	ps := SlotsToLayout(slots)
	if len(ps) != 2 {
		t.Fatalf("slots = %d, want 2", len(ps))
	}

	// ColumnFull → no group, vertical.
	for _, s := range ps {
		if s.Group != "" {
			t.Fatalf("ColumnFull panel %s should have empty group", s.Panel.ID())
		}
		if s.Direction != core.DirVertical {
			t.Fatalf("ColumnFull panel %s should be DirVertical", s.Panel.ID())
		}
	}
}

func TestSlotsToLayout_TwoColumns(t *testing.T) {
	slots := core.Slots{
		{Panel: sp("output"), Weight: 2, Column: core.ColumnMain, Focusable: true},
		{Panel: sp("sidebar"), Weight: 1, Column: core.ColumnSide, Focusable: true},
		{Panel: sp("input"), Focusable: true},
		{Panel: sp("dashboard"), Focusable: true},
	}

	ps := SlotsToLayout(slots)
	if len(ps) != 4 {
		t.Fatalf("slots = %d, want 4", len(ps))
	}

	// Main and Side should be in the same group, horizontal.
	if ps[0].Group != columnGroup {
		t.Fatal("main panel should be in column group")
	}
	if ps[1].Group != columnGroup {
		t.Fatal("side panel should be in column group")
	}
	if ps[0].Direction != core.DirHorizontal {
		t.Fatal("main panel should be DirHorizontal")
	}
	if ps[1].Direction != core.DirHorizontal {
		t.Fatal("side panel should be DirHorizontal")
	}

	// Input and Dashboard should NOT be in the column group.
	if ps[2].Group != "" {
		t.Fatal("input should have empty group")
	}
	if ps[3].Group != "" {
		t.Fatal("dashboard should have empty group")
	}
}

func TestSlotsToLayout_PreservesMetadata(t *testing.T) {
	visible := func() bool { return true }
	slots := core.Slots{
		{Panel: sp("flex"), Weight: 3, MinHeight: 10, Visible: visible, Focusable: true},
		{Panel: sp("fixed"), Focusable: false},
	}

	ps := SlotsToLayout(slots)

	if ps[0].Weight != 3 {
		t.Fatalf("weight = %d, want 3", ps[0].Weight)
	}
	if ps[0].MinHeight != 10 {
		t.Fatalf("minHeight = %d, want 10", ps[0].MinHeight)
	}
	if ps[0].Visible == nil {
		t.Fatal("visible should be preserved")
	}
	if !ps[0].Focusable {
		t.Fatal("focusable should be true")
	}
	if ps[1].Focusable {
		t.Fatal("second slot should not be focusable")
	}
}

func TestSlotsToLayout_BorderMapping(t *testing.T) {
	slots := core.Slots{
		{Panel: sp("default")},
		{Panel: sp("only"), Border: core.SlotBorderOnly},
		{Panel: sp("none"), Border: core.SlotBorderNone},
	}

	ps := SlotsToLayout(slots)

	if ps[0].Border != BorderFocusDepth {
		t.Fatalf("default border = %d, want BorderFocusDepth", ps[0].Border)
	}
	if ps[1].Border != BorderOnly {
		t.Fatalf("only border = %d, want BorderOnly", ps[1].Border)
	}
	if ps[2].Border != BorderNone {
		t.Fatalf("none border = %d, want BorderNone", ps[2].Border)
	}
}

func TestSetSlots_ReplacesAll(t *testing.T) {
	fm := core.NewFocusManager()
	e := NewLayoutEngine(fm, nil)

	e.Register(PanelSlot{Panel: sp("old1")})
	e.Register(PanelSlot{Panel: sp("old2")})

	if len(e.VisibleSlots()) != 2 {
		t.Fatalf("before: %d slots", len(e.VisibleSlots()))
	}

	e.SetSlots([]PanelSlot{
		{Panel: sp("new1")},
	})

	if len(e.VisibleSlots()) != 1 {
		t.Fatalf("after: %d slots, want 1", len(e.VisibleSlots()))
	}
	if e.VisibleSlots()[0].Panel.ID() != "new1" {
		t.Fatal("slot should be new1")
	}
}

package core

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewMode_String(t *testing.T) {
	tests := []struct {
		mode ViewMode
		want string
	}{
		{ViewConversation, "conversation"},
		{ViewPlan, "plan"},
		{ViewAgents, "agents"},
		{ViewDebug, "debug"},
		{ViewDashboard, "dashboard"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("ViewMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestParseViewMode(t *testing.T) {
	tests := []struct {
		input string
		want  ViewMode
		ok    bool
	}{
		{"conversation", ViewConversation, true},
		{"chat", ViewConversation, true},
		{"plan", ViewPlan, true},
		{"agents", ViewAgents, true},
		{"debug", ViewDebug, true},
		{"dashboard", ViewDashboard, true},
		{"dash", ViewDashboard, true},
		{"invalid", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseViewMode(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseViewMode(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestStubView_ImplementsView(t *testing.T) {
	var _ View = (*StubView)(nil)
}

func TestViewRouter_ModeSwitch(t *testing.T) {
	r := NewViewRouter()

	conv := &StubView{ViewID: "conversation"}
	plan := &StubView{ViewID: "plan"}
	r.Register(ViewConversation, conv)
	r.Register(ViewPlan, plan)

	if r.Mode() != ViewConversation {
		t.Fatalf("initial mode = %s, want conversation", r.Mode())
	}

	if !r.SetMode(ViewPlan) {
		t.Fatal("SetMode(ViewPlan) should succeed")
	}
	if r.Mode() != ViewPlan {
		t.Fatalf("mode after switch = %s, want plan", r.Mode())
	}

	if r.SetMode(ViewDebug) {
		t.Fatal("SetMode(ViewDebug) should fail — no view registered")
	}
	if r.Mode() != ViewPlan {
		t.Fatal("mode should stay plan after failed switch")
	}
}

func TestViewRouter_Slots(t *testing.T) {
	r := NewViewRouter()
	p1 := &stubPanel{id: "p1"}
	p2 := &stubPanel{id: "p2"}
	stub := &StubView{ViewID: "test", ViewSlots: Slots{
		{Panel: p1, Weight: 1, Focusable: true},
		{Panel: p2, Column: ColumnSide, Focusable: true},
	}}
	r.Register(ViewConversation, stub)

	slots := r.Slots()
	if len(slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(slots))
	}
	if slots[0].Panel.ID() != "p1" || slots[1].Panel.ID() != "p2" {
		t.Fatal("panel IDs don't match")
	}
	if slots[1].Column != ColumnSide {
		t.Fatal("expected p2 in sidebar column")
	}
}

func TestViewRouter_Panels(t *testing.T) {
	r := NewViewRouter()
	p1 := &stubPanel{id: "p1"}
	stub := &StubView{ViewID: "test", ViewSlots: Slots{
		{Panel: p1, Weight: 1, Focusable: true},
	}}
	r.Register(ViewConversation, stub)

	panels := r.Panels()
	if len(panels) != 1 {
		t.Fatalf("panels = %d, want 1", len(panels))
	}
	if panels[0].ID() != "p1" {
		t.Fatal("panel ID mismatch")
	}
}

func TestViewRouter_NilSlots(t *testing.T) {
	r := NewViewRouter()
	// No views registered — Slots() should return nil.
	if r.Slots() != nil {
		t.Fatal("expected nil slots for unregistered mode")
	}
	if r.Panels() != nil {
		t.Fatal("expected nil panels for unregistered mode")
	}
}

// stubPanel implements Panel for router tests.
type stubPanel struct {
	id string
}

func (p *stubPanel) ID() string                        { return p.id }
func (p *stubPanel) Focused() bool                     { return false }
func (p *stubPanel) SetFocus(_ bool)                   {}
func (p *stubPanel) Children() []Panel                 { return nil }
func (p *stubPanel) Height() int                       { return 1 }
func (p *stubPanel) Collapsible() bool                 { return false }
func (p *stubPanel) Collapsed() bool                   { return false }
func (p *stubPanel) Toggle()                           {}
func (p *stubPanel) Update(_ tea.Msg) (Panel, tea.Cmd) { return p, nil }
func (p *stubPanel) View(_ int) string                 { return p.id }

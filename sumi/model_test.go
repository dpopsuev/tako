package sumi

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func testCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall", Approach: "rapid"},
			{Name: "triage", Approach: "analytical"},
			{Name: "investigate", Approach: "methodical"},
			{Name: "report", Approach: "rigorous"},
		},
		Edges: []circuit.EdgeDef{
			{From: "recall", To: "triage"},
			{From: "triage", To: "investigate"},
			{From: "investigate", To: "report"},
		},
		Start: "recall",
		Done:  "report",
	}
}

func TestModel_Init(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil Cmd for store subscription")
	}
}

func TestModel_DiffMsg_AutoSelectsActiveNode(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	if m.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.selected)
	}

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1",
	})

	diff := view.StateDiff{
		Type:      view.DiffNodeState,
		Node:      "triage",
		State:     view.NodeActive,
		Timestamp: time.Now(),
	}

	updated, _ := m.Update(DiffMsg(diff))
	um := updated.(Model)

	if um.nodeOrder[um.selected] != "triage" {
		t.Errorf("selected = %q, want triage", um.nodeOrder[um.selected])
	}
}

func TestModel_KeyQ_Quits(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("pressing 'q' should return a quit command")
	}
}

func TestModel_TabCyclesNodes(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	for i := 0; i < len(def.Nodes)+1; i++ {
		expected := (i + 1) % len(def.Nodes)
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.selected != expected {
			t.Errorf("after tab %d: selected = %d, want %d", i+1, m.selected, expected)
		}
	}
}

func TestModel_EnterTogglesInspector(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	if m.inspecting {
		t.Fatal("should not be inspecting initially")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.inspecting {
		t.Error("Enter should toggle inspecting on")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.inspecting {
		t.Error("Enter again should toggle inspecting off")
	}
}

func TestModel_SearchSelectsNode(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	// Enter search mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if !m.searching {
		t.Fatal("'/' should enter search mode")
	}

	for _, ch := range "inv" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	// Press Enter to apply search
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.nodeOrder[m.selected] != "investigate" {
		t.Errorf("search for 'inv' selected %q, want investigate", m.nodeOrder[m.selected])
	}
	if m.searching {
		t.Error("search mode should be off after Enter")
	}
}

func TestModel_View_IncludesStatusBar(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	// Simulate window size
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	v := m.View()
	if v == "" {
		t.Fatal("View returned empty")
	}
	hasAny := strings.Contains(v, "Kami:") || strings.Contains(v, "Selected:") || strings.Contains(v, "recall")
	if !hasAny {
		t.Error("View missing status bar elements")
	}
}

func TestSumiRenderer_ImplementsInterface(t *testing.T) {
	var _ view.CircuitRenderer = (*SumiRenderer)(nil)
}

// TestModel_EmptyNodeOrder_NoPanic verifies that all key handlers
// are safe when nodeOrder is empty (Sumi started before any session).
// Reproduces the panic: "index out of range [0] with length 0"
// in toggleBreakpoint when the user presses 'b' on an empty circuit.
func TestModel_EmptyNodeOrder_NoPanic(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}
	store := view.NewCircuitStore(emptyDef)
	defer store.Close()

	m := New(Config{
		Def:    emptyDef,
		Store:  store,
		Layout: view.CircuitLayout{},
		Opts:   RenderOpts{NoColor: true},
	})
	m.Init()

	// Set window size so View() works.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	// Every key that touches m.nodeOrder[m.selected] must not panic.
	dangerousKeys := []string{
		"b",         // toggleBreakpoint
		"tab",       // cycle forward
		"shift+tab", // cycle backward
		"enter",     // toggle inspector
		"up",        // findAdjacentNode
		"down",
		"left",
		"right",
	}

	for _, key := range dangerousKeys {
		t.Run(key, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on key %q with empty nodeOrder: %v", key, r)
				}
			}()

			var msg tea.KeyMsg
			switch key {
			case "tab":
				msg = tea.KeyMsg{Type: tea.KeyTab}
			case "shift+tab":
				msg = tea.KeyMsg{Type: tea.KeyShiftTab}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "up":
				msg = tea.KeyMsg{Type: tea.KeyUp}
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			case "left":
				msg = tea.KeyMsg{Type: tea.KeyLeft}
			case "right":
				msg = tea.KeyMsg{Type: tea.KeyRight}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			}
			m.Update(msg)
		})
	}

	// View() should also not panic with empty nodeOrder.
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string with empty nodeOrder")
	}
}

// TestViewWarRoom_ANSI_Alignment verifies that the War Room layout
// does not corrupt when panels contain ANSI-styled text.
// The rune-canvas approach treats ANSI escape codes as visible characters,
// breaking alignment of all subsequent content.
func TestViewWarRoom_ANSI_Alignment(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "C02",
	})

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: false},
	})

	width, height := 140, 40
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(Model)

	v := m.View()
	if v == "" {
		t.Fatal("View returned empty")
	}

	lines := strings.Split(v, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w > width {
			t.Errorf("line %d: visual width %d > terminal width %d",
				i, w, width)
		}
	}

	if len(lines) != height {
		t.Errorf("output has %d lines, want %d", len(lines), height)
	}

	stripped := ansi.Strip(v)
	for _, panel := range []string{"Circuit", "Workers", "Inspector", "Events"} {
		if !strings.Contains(stripped, panel) {
			t.Errorf("missing panel title %q in stripped output", panel)
		}
	}

	if !strings.Contains(stripped, "C01") {
		t.Error("walker C01 content missing from output")
	}
	if !strings.Contains(stripped, "C02") {
		t.Error("walker C02 content missing from output")
	}
}

func TestPadOrTruncate_ANSIAware(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	styled := lipgloss.NewStyle().Bold(true).Render("hello")
	if len([]rune(styled)) == len("hello") {
		t.Skip("lipgloss did not produce ANSI codes (no color profile)")
	}

	result := padOrTruncate(styled, 10)
	w := lipgloss.Width(result)
	if w != 10 {
		t.Errorf("padOrTruncate visual width = %d, want 10 (rune len = %d)", w, len([]rune(result)))
	}

	long := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("this is a very long styled string")
	truncated := padOrTruncate(long, 8)
	tw := lipgloss.Width(truncated)
	if tw != 8 {
		t.Errorf("truncated visual width = %d, want 8", tw)
	}
}

// TestRenderTimelineContent_NewestFirst verifies that the timeline content
// shows newest events at the top so that when the panel clips to innerH,
// the most recent events are visible.
func TestRenderTimelineContent_NewestFirst(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	now := time.Now()
	for i := 0; i < 50; i++ {
		m.timeline.Push(TimelineEntry{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Walker:    "w1",
			Type:      view.DiffNodeState,
			Node:      fmt.Sprintf("node-%03d", i),
		})
	}

	content := m.renderTimelineContent()
	lines := strings.Split(content, "\n")

	if len(lines) < 8 {
		t.Fatalf("expected at least 8 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "node-049") {
		t.Errorf("first line should contain newest event (node-049), got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "node-048") {
		t.Errorf("second line should contain second-newest event (node-048), got: %s", lines[1])
	}
}

// TestCasesPanel_ShowsAllCases verifies that the Cases panel shows all cases
// across the full lifecycle (active, completed, error), not just active walkers.
func TestCasesPanel_ShowsAllCases(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	for i := 1; i <= 12; i++ {
		caseID := fmt.Sprintf("C%02d", i)
		store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeEnter, Node: "recall", Walker: caseID,
		})
	}

	for i := 1; i <= 4; i++ {
		caseID := fmt.Sprintf("C%02d", i)
		store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventWalkComplete, Walker: caseID,
		})
	}

	for i := 5; i <= 6; i++ {
		caseID := fmt.Sprintf("C%02d", i)
		store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventWalkError, Walker: caseID, Error: fmt.Errorf("timeout"),
		})
	}

	snap := store.Snapshot()
	if len(snap.Cases) != 12 {
		t.Fatalf("expected 12 cases, got %d", len(snap.Cases))
	}

	completed := 0
	errored := 0
	active := 0
	for _, ci := range snap.Cases {
		switch ci.State {
		case view.CaseCompleted:
			completed++
		case view.CaseError:
			errored++
		case view.CaseActive:
			active++
		}
	}

	if completed != 4 {
		t.Errorf("completed cases = %d, want 4", completed)
	}
	if errored != 2 {
		t.Errorf("errored cases = %d, want 2", errored)
	}
	if active != 6 {
		t.Errorf("active cases = %d, want 6", active)
	}

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	content := m.renderCasesContent()
	for i := 1; i <= 12; i++ {
		caseID := fmt.Sprintf("C%02d", i)
		if !strings.Contains(content, caseID) {
			t.Errorf("cases content missing case %s", caseID)
		}
	}

	if !strings.Contains(content, "✓") {
		t.Error("missing completed icon (✓)")
	}
	if !strings.Contains(content, "✗") {
		t.Error("missing error icon (✗)")
	}
	if !strings.Contains(content, "▶") {
		t.Error("missing active icon (▶)")
	}
}

// TestRenderGraph_CenteredInPanel verifies that the graph is centered
// within the circuit panel when the graph is smaller than the panel.
func TestRenderGraph_CenteredInPanel(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	graphStr := RenderGraph(def, layout, store.Snapshot(), RenderOpts{NoColor: true})
	graphLines := strings.Split(graphStr, "\n")

	graphW := 0
	for _, line := range graphLines {
		if w := lipgloss.Width(line); w > graphW {
			graphW = w
		}
	}

	panelW := graphW + 40
	panelH := len(graphLines) + 20
	rect := Rect{0, 0, panelW, panelH}

	innerW := panelW - 2
	innerH := panelH - 2
	centered := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, graphStr)
	frame := RenderPanelFrame("Circuit", centered, rect, false, true)

	frameLines := strings.Split(frame, "\n")
	if len(frameLines) < 3 {
		t.Fatalf("expected at least 3 frame lines, got %d", len(frameLines))
	}

	contentStartLine := frameLines[1]
	trimmed := strings.TrimLeft(contentStartLine, "│ ")
	if strings.TrimSpace(trimmed) != "" {
		t.Logf("first content line non-empty: %q (graph starts later = centered vertically)", trimmed)
	}

	expectedHPad := (innerW - graphW) / 2
	if expectedHPad <= 0 {
		t.Skip("graph fills panel, no centering to verify")
	}

	foundCentered := false
	for _, line := range frameLines[1 : len(frameLines)-1] {
		stripped := strings.TrimPrefix(line, "│")
		if stripped == "" {
			continue
		}
		leadingSpaces := len(stripped) - len(strings.TrimLeft(stripped, " "))
		if leadingSpaces >= expectedHPad-1 && strings.TrimSpace(stripped) != "" {
			foundCentered = true
			break
		}
	}
	if !foundCentered {
		t.Errorf("graph does not appear horizontally centered (expected ~%d leading spaces)", expectedHPad)
	}
}

// TestAnimation_SpinnerAdvances verifies that the state indicator for active
// nodes cycles through braille spinner chars based on the animation frame.
func TestAnimation_SpinnerAdvances(t *testing.T) {
	seen := make(map[string]bool)
	for frame := 1; frame <= 20; frame++ {
		icon := stateIndicator(view.NodeActive, frame)
		seen[icon] = true
	}

	if len(seen) < 3 {
		t.Errorf("expected at least 3 distinct spinner chars, got %d: %v", len(seen), seen)
	}

	staticIcon := stateIndicator(view.NodeActive, 0)
	if staticIcon != "▶" {
		t.Errorf("frame 0 should use static indicator ▶, got %q", staticIcon)
	}

	completedIcon := stateIndicator(view.NodeCompleted, 5)
	if completedIcon != "✓" {
		t.Errorf("completed nodes should always show ✓ regardless of frame, got %q", completedIcon)
	}
}

// TestAnimation_EdgeHighlightDecays verifies that edge highlight counters
// decrement each tick and are cleaned up when reaching zero.
func TestAnimation_EdgeHighlightDecays(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	m.edgeHighlights["recall->triage"] = 3

	for tick := 0; tick < 3; tick++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}

	if _, exists := m.edgeHighlights["recall->triage"]; exists {
		t.Errorf("edge highlight should be removed after 3 ticks, still has %d frames",
			m.edgeHighlights["recall->triage"])
	}
}

// TestDSBadge_UnicodeSymbols verifies that D/S badges use OSS unicode symbols
// instead of bracket notation.
func TestDSBadge_UnicodeSymbols(t *testing.T) {
	tests := []struct {
		transformer string
		want        string
	}{
		{"core.jq", "⚙"},
		{"core.file", "⚙"},
		{"core.template", "⚙"},
		{"core.noop", "⚙"},
		{"core.passthrough", "⚙"},
		{"core.dialectic", "Δ"},
		{"some.llm.transformer", "✦"},
		{"", ""},
	}
	for _, tt := range tests {
		got := DSBadge(tt.transformer)
		if got != tt.want {
			t.Errorf("DSBadge(%q) = %q, want %q", tt.transformer, got, tt.want)
		}
	}
}

// TestDrawNode_ElementColoredBorders verifies that active nodes use element color
// for border styling rather than generic state-only color.
func TestDrawNode_ElementColoredBorders(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})

	snap := store.Snapshot()
	opts := RenderOpts{NoColor: false, AnimFrame: 1}
	graphStr := RenderGraph(def, layout, snap, opts)

	if !strings.Contains(graphStr, "recall") {
		t.Error("graph should contain node name 'recall'")
	}

	noColorOpts := RenderOpts{NoColor: true}
	noColorGraph := RenderGraph(def, layout, snap, noColorOpts)
	if !strings.Contains(noColorGraph, "recall") {
		t.Error("noColor graph should contain node name 'recall'")
	}

	if graphStr == noColorGraph {
		t.Error("colored graph should differ from noColor graph (element styling)")
	}
}

// --- DiffReset tests ---
// These tests verify that the Model correctly rebuilds its rendering
// state (def, layout, nodeOrder) when the store emits a DiffReset event,
// which happens on SSE reconnect after a session swap.

// TestModel_DiffReset_EmptyToPopulated verifies the core bug:
// Model starts with an empty def (Sumi launched before any session),
// then receives DiffReset from store.Reset with a real circuit def.
// The Model should rebuild def, layout, nodeOrder so the circuit renders.
func TestModel_DiffReset_EmptyToPopulated(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}
	store := view.NewCircuitStore(emptyDef)
	defer store.Close()

	emptyLayout := view.CircuitLayout{}

	m := New(Config{
		Def:    emptyDef,
		Store:  store,
		Layout: emptyLayout,
		Opts:   RenderOpts{NoColor: true},
	})
	m.Init()

	// Verify initial state: empty circuit.
	if len(m.nodeOrder) != 0 {
		t.Fatalf("initial nodeOrder should be empty, got %d", len(m.nodeOrder))
	}
	if len(m.layout.Grid) != 0 {
		t.Fatalf("initial layout.Grid should be empty, got %d", len(m.layout.Grid))
	}

	// Simulate what rebootstrapStore does: Reset the store with a real def.
	realDef := testCircuit()
	store.Reset(realDef)

	// Feed DiffReset through the Model's Update loop.
	resetDiff := view.StateDiff{
		Type:      view.DiffReset,
		Timestamp: time.Now(),
	}
	updated, cmd := m.Update(DiffMsg(resetDiff))
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("Update should return a cmd to wait for next diff")
	}

	// After DiffReset, the Model should have rebuilt its rendering state.
	if len(m.nodeOrder) == 0 {
		t.Error("BUG: nodeOrder is still empty after DiffReset — Model does not rebuild on reset")
	}
	if len(m.layout.Grid) == 0 {
		t.Error("BUG: layout.Grid is still empty after DiffReset — Model does not rebuild on reset")
	}
	if len(m.def.Nodes) == 0 {
		t.Error("BUG: def.Nodes is still empty after DiffReset — Model does not rebuild on reset")
	}

	// The circuit should render something other than "(empty circuit)".
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	v := m.View()
	if strings.Contains(v, "(empty circuit)") {
		t.Error("BUG: View still shows '(empty circuit)' after DiffReset with populated store")
	}

	t.Logf("after DiffReset: nodeOrder=%d, layout.Grid=%d, def.Nodes=%d",
		len(m.nodeOrder), len(m.layout.Grid), len(m.def.Nodes))
}

// TestModel_DiffReset_PreservesEventCount verifies that DiffReset doesn't
// zero the event counter — it should increment like any other diff.
func TestModel_DiffReset_PreservesEventCount(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})
	m.Init()

	// Send a few normal diffs.
	for i := 0; i < 5; i++ {
		store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1",
		})
		diff := view.StateDiff{Type: view.DiffNodeState, Node: "recall", State: view.NodeActive, Timestamp: time.Now()}
		updated, _ := m.Update(DiffMsg(diff))
		m = updated.(Model)
	}

	countBefore := m.eventCount

	// Now a DiffReset.
	resetDiff := view.StateDiff{Type: view.DiffReset, Timestamp: time.Now()}
	updated, _ := m.Update(DiffMsg(resetDiff))
	m = updated.(Model)

	if m.eventCount != countBefore+1 {
		t.Errorf("eventCount = %d, want %d (DiffReset should increment, not reset)", m.eventCount, countBefore+1)
	}
}

// TestModel_SessionStartAfterWatch simulates the real-world scenario:
// Sumi starts watching before any session → empty circuit. Then a session
// starts, the store gets populated via SSE events. Verifies the Model
// eventually shows the circuit.
func TestModel_SessionStartAfterWatch(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}
	store := view.NewCircuitStore(emptyDef)
	defer store.Close()

	m := New(Config{
		Def:    emptyDef,
		Store:  store,
		Layout: view.CircuitLayout{},
		Opts:   RenderOpts{NoColor: true},
	})
	m.Init()

	// Set window size so View() renders.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	// Verify: "(empty circuit)" initially.
	v := m.View()
	if !strings.Contains(v, "(empty circuit)") {
		t.Fatal("expected '(empty circuit)' before session start")
	}

	// Now simulate session start: store.Reset with a real circuit def,
	// followed by node/walker events (mimicking rebootstrapStore + SSE).
	realDef := testCircuit()
	store.Reset(realDef)

	// Feed the DiffReset.
	resetDiff := view.StateDiff{Type: view.DiffReset, Timestamp: time.Now()}
	updated, _ = m.Update(DiffMsg(resetDiff))
	m = updated.(Model)

	// Feed walker events through the store (as real SSE flow does).
	walkEvents := []circuit.WalkEvent{
		{Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01"},
		{Type: circuit.EventNodeEnter, Node: "triage", Walker: "C02"},
	}
	for _, we := range walkEvents {
		store.OnEvent(we)
	}

	// Now feed the resulting DiffMsgs to the Model (Bubble Tea Update loop).
	diffs := []view.StateDiff{
		{Type: view.DiffNodeState, Node: "recall", State: view.NodeActive, Timestamp: time.Now()},
		{Type: view.DiffWalkerAdded, Node: "recall", Walker: "C01", Timestamp: time.Now()},
		{Type: view.DiffNodeState, Node: "triage", State: view.NodeActive, Timestamp: time.Now()},
		{Type: view.DiffWalkerAdded, Node: "triage", Walker: "C02", Timestamp: time.Now()},
	}
	for _, diff := range diffs {
		updated, _ = m.Update(DiffMsg(diff))
		m = updated.(Model)
	}

	// Now the Model should show the circuit, not "(empty circuit)".
	v = m.View()
	if strings.Contains(v, "(empty circuit)") {
		t.Error("BUG: View still shows '(empty circuit)' after session start + DiffReset + events")
	}

	// Should have nodes in nodeOrder.
	if len(m.nodeOrder) == 0 {
		t.Error("BUG: nodeOrder still empty after session start")
	}

	// Snapshot should reflect the store state.
	if len(m.snap.Walkers) < 2 {
		t.Errorf("snap.Walkers = %d, want >= 2 (C01, C02)", len(m.snap.Walkers))
	}

	t.Logf("after session start: nodeOrder=%d, walkers=%d, view contains circuit=%v",
		len(m.nodeOrder), len(m.snap.Walkers), !strings.Contains(m.View(), "(empty circuit)"))
}

// TestModel_InitSubscription_SurvivesUpdateLoop verifies that the store
// subscription created in Init() persists across the Bubble Tea Update loop.
//
// BUG reproduced: Init() has a value receiver, so m.subCh is set on a copy.
// The first Cmd's closure captures the copy's channel (non-nil) and delivers
// one diff. But Update returns waitForDiff(m.subCh) using the Program's model
// where subCh is nil. Reading from nil blocks forever — the TUI freezes after
// exactly 1 diff.
func TestModel_InitSubscription_SurvivesUpdateLoop(t *testing.T) {
	def := testCircuit()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
		Opts:   RenderOpts{NoColor: true},
	})

	// Simulate Bubble Tea's lifecycle:
	// 1. Init() → returns a batch Cmd (diff waiter + tick)
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() returned nil Cmd")
	}

	// 2. Send an event to the store so the channel has a message.
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1",
	})

	// 3. Execute the batch Cmd and find the DiffMsg among the results.
	batchResult := initCmd()
	var msg tea.Msg
	if batch, ok := batchResult.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			if cmd == nil {
				continue
			}
			result := cmd()
			if _, isDiff := result.(DiffMsg); isDiff {
				msg = result
				break
			}
		}
	} else if _, isDiff := batchResult.(DiffMsg); isDiff {
		msg = batchResult
	}
	if msg == nil {
		t.Fatal("no DiffMsg found in Init() batch result")
	}
	if _, ok := msg.(DiffMsg); !ok {
		t.Fatalf("first Cmd returned %T, want DiffMsg", msg)
	}

	// 4. Pass the DiffMsg to Update (simulates Program.Update).
	//    With the fix, subCh is set in New(), so it persists across the loop.
	updated, nextCmd := m.Update(msg)

	// 5. The nextCmd should be non-nil (waiting for next diff).
	if nextCmd == nil {
		t.Fatal("Update returned nil Cmd — no further diffs will be received")
	}

	// 6. Send another event.
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1",
	})

	// 7. Execute the next Cmd — it should deliver the second diff.
	//    If subCh is nil, this blocks forever (the bug).
	done := make(chan tea.Msg, 1)
	go func() {
		done <- nextCmd()
	}()

	select {
	case msg2 := <-done:
		if _, ok := msg2.(DiffMsg); !ok {
			t.Fatalf("second Cmd returned %T, want DiffMsg", msg2)
		}
		t.Log("second diff received — subscription survives Update loop")
	case <-time.After(2 * time.Second):
		t.Fatal("BUG: second Cmd blocked forever — m.subCh is nil after Init() value receiver")
	}

	_ = updated
}

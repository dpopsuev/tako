package sumi

import (
	"strings"
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
)

func testInspectorCircuit() (*framework.CircuitDef, *view.CircuitSnapshot) {
	def := &framework.CircuitDef{
		Circuit: "test",
		HandlerType: "transformer",
		Nodes: []framework.NodeDef{
			{Name: "recall", Approach: "rapid", Handler: "core.llm", Description: "Recall phase retrieves candidate issues from RP"},
			{Name: "triage", Approach: "analytical", Handler: "core.jq"},
			{Name: "report", Approach: "rigorous"},
		},
		Edges: []framework.EdgeDef{
			{From: "recall", To: "triage", When: "true"},
			{From: "triage", To: "report", When: "score > 0.8"},
		},
		Zones: map[string]framework.ZoneDef{
			"analysis": {Approach: "rapid", Nodes: []string{"recall", "triage"}},
		},
	}

	snap := &view.CircuitSnapshot{
		Nodes: map[string]view.NodeState{
			"recall": {Name: "recall", State: view.NodeCompleted, Zone: "analysis", Element: "fire"},
			"triage": {Name: "triage", State: view.NodeActive, Zone: "analysis", Element: "water"},
			"report": {Name: "report", State: view.NodeIdle, Element: "diamond"},
		},
		Walkers: map[string]view.WalkerPosition{
			"w-01": {WalkerID: "w-01", Node: "triage", Element: "water"},
		},
	}
	return def, snap
}

func TestInspectorPanel_ID(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	if p.ID() != "inspector" {
		t.Errorf("ID = %q, want inspector", p.ID())
	}
}

func TestInspectorPanel_NoSelection(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)

	content := p.View(Rect{0, 0, 40, 20})
	if !strings.Contains(content, "Select a node") {
		t.Errorf("empty inspector should prompt selection, got: %s", content)
	}
}

func TestInspectorPanel_ShowsNodeDetails(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")

	content := p.View(Rect{0, 0, 40, 30})

	checks := []string{"recall", "rapid", "completed", "core.llm", "transformer", "✦", "analysis"}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("inspector should contain %q, got:\n%s", want, content)
		}
	}
}

func TestInspectorPanel_ShowsDescription(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")

	content := p.View(Rect{0, 0, 50, 30})

	if !strings.Contains(content, "Recall phase") {
		t.Errorf("inspector should show description, got:\n%s", content)
	}
}

func TestInspectorPanel_ShowsWalker(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("triage")

	content := p.View(Rect{0, 0, 40, 30})

	if !strings.Contains(content, "w-01") {
		t.Errorf("inspector should show walker at node, got:\n%s", content)
	}
}

func TestInspectorPanel_ShowsEdges(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("triage")

	content := p.View(Rect{0, 0, 50, 30})

	if !strings.Contains(content, "recall") {
		t.Errorf("inspector should show incoming edge from recall, got:\n%s", content)
	}
	if !strings.Contains(content, "report") {
		t.Errorf("inspector should show outgoing edge to report, got:\n%s", content)
	}
	if !strings.Contains(content, "score > 0.8") {
		t.Errorf("inspector should show edge condition, got:\n%s", content)
	}
}

func TestInspectorPanel_ShowsZoneMembers(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("triage")

	content := p.View(Rect{0, 0, 40, 30})

	if !strings.Contains(content, "analysis") {
		t.Errorf("inspector should show zone name, got:\n%s", content)
	}
}

func TestInspectorPanel_DeterministicBadge(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("triage")

	content := p.View(Rect{0, 0, 40, 30})

	if !strings.Contains(content, "⚙") {
		t.Errorf("triage (core.jq) should show ⚙ badge, got:\n%s", content)
	}
}

func TestInspectorPanel_SetNodeResetsScroll(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")
	p.scrollY = 5

	p.SetNode("triage")
	if p.scrollY != 0 {
		t.Errorf("SetNode should reset scroll to 0, got %d", p.scrollY)
	}
}

func TestInspectorPanel_ScrollDown(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")

	p.View(Rect{0, 0, 40, 5})

	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.scrollY != 1 {
		t.Errorf("scroll after down = %d, want 1", p.scrollY)
	}

	p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.scrollY != 0 {
		t.Errorf("scroll after up = %d, want 0", p.scrollY)
	}
}

func TestInspectorPanel_ScrollUpClamps(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")

	p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.scrollY != 0 {
		t.Errorf("scroll should not go below 0, got %d", p.scrollY)
	}
}

func TestInspectorPanel_Home(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("recall")
	p.scrollY = 10

	p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.scrollY != 0 {
		t.Errorf("Home should reset scroll to 0, got %d", p.scrollY)
	}
}

func TestInspectorPanel_UnknownNode(t *testing.T) {
	def, snap := testInspectorCircuit()
	p := NewInspectorPanel(def, snap, true)
	p.SetNode("nonexistent")

	content := p.View(Rect{0, 0, 40, 20})
	if !strings.Contains(content, "not found") {
		t.Errorf("unknown node should show not found, got: %s", content)
	}
}

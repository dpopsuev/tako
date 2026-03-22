package circuit

import (
	"strings"
	"testing"
)

func TestRender_FlatGraph(t *testing.T) {
	def := &CircuitDef{
		Circuit: "flat",
		Nodes: []NodeDef{
			{Name: "a"},
			{Name: "b"},
		},
		Edges: []EdgeDef{
			{ID: "E1", Name: "a-to-b", From: "a", To: "b"},
			{ID: "E2", Name: "b-done", From: "b", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}

	result := Render(def)

	if !strings.Contains(result, "graph LR") {
		t.Error("missing graph LR header")
	}
	if !strings.Contains(result, `a -->|"E1: a-to-b"| b`) {
		t.Errorf("missing edge E1 in output:\n%s", result)
	}
	if !strings.Contains(result, `b -->|"E2: b-done"| _done`) {
		t.Errorf("missing edge E2 in output:\n%s", result)
	}
	if strings.Contains(result, "subgraph") {
		t.Error("flat graph should not contain subgraphs")
	}
}

func TestRender_WithZones(t *testing.T) {
	def := &CircuitDef{
		Circuit: "zoned",
		Nodes: []NodeDef{
			{Name: "recall"},
			{Name: "triage"},
			{Name: "report"},
		},
		Edges: []EdgeDef{
			{ID: "H1", Name: "recall-triage", From: "recall", To: "triage"},
			{ID: "H2", Name: "triage-report", From: "triage", To: "report"},
			{ID: "H3", Name: "report-done", From: "report", To: "_done"},
		},
		Zones: map[string]ZoneDef{
			"intake": {Nodes: []string{"recall", "triage"}},
			"output": {Nodes: []string{"report"}},
		},
		Start: "a",
		Done:  "_done",
	}

	result := Render(def)

	if !strings.Contains(result, "subgraph intake [Intake]") {
		t.Errorf("missing intake subgraph in output:\n%s", result)
	}
	if !strings.Contains(result, "subgraph output [Output]") {
		t.Errorf("missing output subgraph in output:\n%s", result)
	}
	if !strings.Contains(result, "recall") {
		t.Error("missing recall node")
	}
	if !strings.Contains(result, "end") {
		t.Error("missing subgraph end")
	}
}

func TestRender_EdgeLabelFallback(t *testing.T) {
	def := &CircuitDef{
		Circuit: "fallback",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
		},
	}

	result := Render(def)
	if !strings.Contains(result, `"E1: E1"`) {
		t.Errorf("edge with no name should use ID as label:\n%s", result)
	}
}

func TestRender_HyphenatedNodes(t *testing.T) {
	def := &CircuitDef{
		Circuit: "hyphens",
		Nodes:    []NodeDef{{Name: "cross-examine"}, {Name: "counter-investigate"}},
		Edges: []EdgeDef{
			{ID: "D1", Name: "proceed", From: "cross-examine", To: "counter-investigate"},
		},
	}

	result := Render(def)
	if strings.Contains(result, "cross-examine") {
		t.Errorf("hyphens should be replaced with underscores in Mermaid IDs:\n%s", result)
	}
	if !strings.Contains(result, "cross_examine") {
		t.Errorf("expected cross_examine in output:\n%s", result)
	}
}

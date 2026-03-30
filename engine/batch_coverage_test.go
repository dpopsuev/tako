package engine

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestDiagnoseNodeCoverage_AllVisited(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
		},
	}
	results := []BatchWalkResult{
		{Path: []string{"a", "b"}},
	}
	unvisited := DiagnoseNodeCoverage(def, results)
	if len(unvisited) != 0 {
		t.Errorf("expected no unvisited, got %v", unvisited)
	}
}

func TestDiagnoseNodeCoverage_SomeUnvisited(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
			{Name: "gather-code"},
		},
	}
	results := []BatchWalkResult{
		{Path: []string{"a", "b"}},
		{Path: []string{"a", "b"}},
	}

	logs := captureDiagLogs(func() {
		unvisited := DiagnoseNodeCoverage(def, results)
		if len(unvisited) != 1 {
			t.Errorf("expected 1 unvisited, got %v", unvisited)
		}
		if unvisited[0] != "gather-code" {
			t.Errorf("expected gather-code, got %v", unvisited)
		}
	})

	if !diagContains(logs, "gather-code") {
		t.Errorf("expected warning mentioning gather-code, got:\n%s", logs)
	}
}

func TestDiagnoseNodeCoverage_DoneNodeSkipped(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "done"},
		},
	}
	results := []BatchWalkResult{
		{Path: []string{"a"}},
	}
	// "done" is the terminal node — should not be reported as unvisited.
	unvisited := DiagnoseNodeCoverage(def, results)
	if len(unvisited) != 0 {
		t.Errorf("done node should be skipped, got unvisited: %v", unvisited)
	}
}

func TestDiagnoseNodeCoverage_AcrossCases(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
	}
	// Each case visits different nodes — but together they cover all.
	results := []BatchWalkResult{
		{Path: []string{"a", "b"}},
		{Path: []string{"a", "c"}},
	}
	unvisited := DiagnoseNodeCoverage(def, results)
	if len(unvisited) != 0 {
		t.Errorf("expected all covered across cases, got unvisited: %v", unvisited)
	}
}

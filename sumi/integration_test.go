package sumi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

func TestIntegration_WalkAndRender(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "integration",
		Zones: map[string]circuit.ZoneDef{
			"input":  {Nodes: []string{"recall", "triage"}, Approach: "rapid"},
			"output": {Nodes: []string{"report"}, Approach: "rigorous"},
		},
		Nodes: []circuit.NodeDef{
			{Name: "recall", Approach: "rapid"},
			{Name: "triage", Approach: "analytical"},
			{Name: "report", Approach: "rigorous"},
		},
		Edges: []circuit.EdgeDef{
			{From: "recall", To: "triage"},
			{From: "triage", To: "report"},
		},
		Start: "recall",
		Done:  "report",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}

	// Subscribe to diffs
	_, ch := store.Subscribe()

	// Step 1: walker enters recall
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "sentinel",
	})
	diff := <-ch
	if diff.Type != view.DiffNodeState || diff.Node != "recall" || diff.State != view.NodeActive {
		t.Errorf("step1 diff: got %+v", diff)
	}
	// walker add diff
	<-ch

	snap := store.Snapshot()
	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if !strings.Contains(output, "●") {
		t.Error("step1: expected walker marker ● on recall")
	}
	if !strings.Contains(output, "▶") {
		t.Error("step1: expected active indicator ▶")
	}

	// Step 2: walker exits recall
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "recall",
	})
	diff = <-ch
	if diff.Type != view.DiffNodeState || diff.State != view.NodeCompleted {
		t.Errorf("step2 diff: got %+v", diff)
	}

	// Step 3: walker enters triage
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "sentinel",
	})
	<-ch // node state
	<-ch // walker moved

	snap = store.Snapshot()
	output = RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if !strings.Contains(output, "✓") {
		t.Error("step3: expected completed indicator ✓ for recall")
	}

	// Step 4: walk completes
	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "triage",
	})
	<-ch

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "report", Walker: "sentinel",
	})
	<-ch // node state
	<-ch // walker moved

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "report",
	})
	<-ch

	store.OnEvent(circuit.WalkEvent{
		Type: circuit.EventWalkComplete,
	})
	// walk_complete now clears walkers first, then emits DiffCompleted
	diff = <-ch
	if diff.Type == view.DiffWalkerRemoved {
		diff = <-ch // consume walker removal, read next
	}
	if diff.Type != view.DiffCompleted {
		t.Errorf("final diff: got %+v", diff)
	}

	snap = store.Snapshot()
	if !snap.Completed {
		t.Error("circuit should be marked completed")
	}

	// Zone borders should be present
	output = RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if !strings.Contains(output, "input") {
		t.Error("output missing zone label 'input'")
	}
	if !strings.Contains(output, "output") {
		t.Error("output missing zone label 'output'")
	}
}

func TestIntegration_BreakpointRendering(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "bp-test",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
		Edges: []circuit.EdgeDef{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
		},
		Start: "a",
		Done:  "c",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	store.SetBreakpoints([]string{"b"})
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if !strings.Contains(output, "◉") {
		t.Error("breakpoint marker missing")
	}
}

func TestIntegration_ErrorState(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "error-test",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
		},
		Edges: []circuit.EdgeDef{
			{From: "a", To: "b"},
		},
		Start: "a",
		Done:  "b",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	store.OnEvent(circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Node:  "a",
		Error: fmt.Errorf("transformer failed"),
	})

	snap := store.Snapshot()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if !strings.Contains(output, "✗") {
		t.Error("error indicator missing")
	}
}

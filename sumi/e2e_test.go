package sumi

import (
	"os"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

func TestE2E_NoColorOutput_DialecticCircuit(t *testing.T) {
	data, err := os.ReadFile("../testdata/defect-dialectic.yaml")
	if err != nil {
		t.Fatalf("read circuit: %v", err)
	}
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("load circuit: %v", err)
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}

	snap := store.Snapshot()
	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	// No ANSI escape sequences
	if strings.Contains(output, "\033[") {
		t.Error("--no-color output contains ANSI escape sequences")
	}

	// Box-drawing chars preserved
	for _, ch := range []string{"┌", "┐", "└", "┘", "│", "─"} {
		if !strings.Contains(output, ch) {
			t.Errorf("output missing box-drawing char %q", ch)
		}
	}

	// All nodes present
	for _, nd := range def.Nodes {
		if !strings.Contains(output, nd.Name) {
			t.Errorf("output missing node %q", nd.Name)
		}
	}

	// All zone labels
	for zoneName := range def.Zones {
		if !strings.Contains(output, zoneName) {
			t.Errorf("output missing zone %q", zoneName)
		}
	}

	// Output is parseable (each line is well-formed)
	lines := strings.Split(output, "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines, got %d", len(lines))
	}
}

func TestE2E_NoColorOutput_RCACircuit(t *testing.T) {
	data, err := os.ReadFile("../testdata/rca-investigation.yaml")
	if err != nil {
		t.Fatalf("read circuit: %v", err)
	}
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("load circuit: %v", err)
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}

	snap := store.Snapshot()
	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	if strings.Contains(output, "\033[") {
		t.Error("--no-color output contains ANSI escape sequences")
	}

	for _, nd := range def.Nodes {
		if !strings.Contains(output, nd.Name) {
			t.Errorf("output missing node %q", nd.Name)
		}
	}
}

func TestE2E_StatusLine(t *testing.T) {
	snap := view.CircuitSnapshot{
		CircuitName: "test-circuit",
		Nodes: map[string]view.NodeState{
			"a": {Name: "a", State: view.NodeCompleted},
			"b": {Name: "b", State: view.NodeIdle},
		},
		Completed: true,
	}

	line := renderStatusLine(snap, RenderOpts{NoColor: true})
	if !strings.Contains(line, "test-circuit") {
		t.Error("status line missing circuit name")
	}
	if !strings.Contains(line, "[DONE]") {
		t.Error("status line missing [DONE]")
	}
	if !strings.Contains(line, "Nodes: 2") {
		t.Errorf("status line missing node count: %s", line)
	}
}

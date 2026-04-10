package calibrate

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestWrap_PreservesTopology(t *testing.T) {
	base := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "a", Instrument: "node"},
			{Name: "b", Action: "b", Instrument: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
		},
		Start: "a",
		Done:  "b",
	}

	wrapped := Wrap(base, DecoratorConfig{})

	if len(wrapped.Nodes) != len(base.Nodes) {
		t.Errorf("nodes: got %d, want %d", len(wrapped.Nodes), len(base.Nodes))
	}
	if len(wrapped.Edges) != len(base.Edges) {
		t.Errorf("edges: got %d, want %d", len(wrapped.Edges), len(base.Edges))
	}
	for i := range base.Nodes {
		if wrapped.Nodes[i].Name != base.Nodes[i].Name {
			t.Errorf("node[%d].Name: got %q, want %q", i, wrapped.Nodes[i].Name, base.Nodes[i].Name)
		}
	}
	for i := range base.Edges {
		if wrapped.Edges[i].ID != base.Edges[i].ID {
			t.Errorf("edge[%d].ID: got %q, want %q", i, wrapped.Edges[i].ID, base.Edges[i].ID)
		}
	}
}

func TestWrap_MarksAsCalibration(t *testing.T) {
	base := &circuit.CircuitDef{
		Circuit: "test",
		Nodes:   []circuit.NodeDef{{Name: "a", Action: "a", Instrument: "node"}},
		Edges:   []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
		Start:   "a",
		Done:    "done",
	}

	wrapped := Wrap(base, DecoratorConfig{})

	if !IsCalibrationWrapped(wrapped) {
		t.Error("IsCalibrationWrapped(wrapped): got false, want true")
	}
}

func TestIsCalibrationWrapped_FalseForUnwrapped(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes:   []circuit.NodeDef{{Name: "a", Action: "a", Instrument: "node"}},
		Edges:   []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
		Start:   "a",
		Done:    "done",
	}

	if IsCalibrationWrapped(def) {
		t.Error("IsCalibrationWrapped(plain def): got true, want false")
	}

	// Also verify nil Vars
	def.Vars = nil
	if IsCalibrationWrapped(def) {
		t.Error("IsCalibrationWrapped(def with nil Vars): got true, want false")
	}
}

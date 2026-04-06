package def

import (
	"errors"
	"testing"
)

func TestNormalize_UndefinedEdgeNode(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
			{ID: "a-ghost", From: "a", To: "ghost"},
		},
		Start: "a",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err == nil {
		t.Fatal("expected error for undefined edge to-node, got nil")
	}
	if !errors.Is(err, ErrCircuit) {
		t.Errorf("expected ErrCircuit, got %v", err)
	}
	if testing.Verbose() {
		t.Logf("error: %v", err)
	}
}

func TestNormalize_UndefinedEdgeFromNode(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "ghost-b", From: "ghost", To: "b"},
		},
		Start: "a",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err == nil {
		t.Fatal("expected error for undefined edge from-node, got nil")
	}
	if !errors.Is(err, ErrCircuit) {
		t.Errorf("expected ErrCircuit, got %v", err)
	}
}

func TestNormalize_UndefinedStart(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
		},
		Start: "nonexistent",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err == nil {
		t.Fatal("expected error for undefined start node, got nil")
	}
	if !errors.Is(err, ErrCircuit) {
		t.Errorf("expected ErrCircuit, got %v", err)
	}
}

func TestNormalize_UndefinedZoneNode(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
		},
		Zones: map[string]ZoneDef{
			"zone1": {Nodes: []NodeName{"a", "ghost"}},
		},
		Start: "a",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err == nil {
		t.Fatal("expected error for undefined zone node, got nil")
	}
	if !errors.Is(err, ErrCircuit) {
		t.Errorf("expected ErrCircuit, got %v", err)
	}
}

func TestNormalize_EdgeToDoneIsValid(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
			{ID: "b-done", From: "b", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	cd, err := raw.normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cd.Circuit != "test" {
		t.Errorf("circuit = %q, want test", cd.Circuit)
	}
}

func TestNormalize_ValidCircuit(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
		},
		Zones: map[string]ZoneDef{
			"zone1": {Nodes: []NodeName{"a", "b"}},
		},
		Start: "a",
		Done:  "_done",
	}
	cd, err := raw.normalize()
	if err != nil {
		t.Fatalf("unexpected error for valid circuit: %v", err)
	}
	if len(cd.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(cd.Nodes))
	}
}

func TestNormalize_InlineEdgeToUndefinedNode(t *testing.T) {
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{
				NodeDef: NodeDef{Name: "a"},
				Edges:   rawEdgeList{{To: "ghost"}},
			},
			{NodeDef: NodeDef{Name: "b"}},
		},
		Start: "a",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err == nil {
		t.Fatal("expected error for inline edge to undefined node, got nil")
	}
	if !errors.Is(err, ErrCircuit) {
		t.Errorf("expected ErrCircuit, got %v", err)
	}
}

func TestNormalize_EmptyStartSkipsValidation(t *testing.T) {
	// When start is empty (overlay), no validation.
	raw := rawCircuitDef{
		Circuit: "test",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "a"}},
		},
		Edges: []EdgeDef{},
		Start: "",
		Done:  "",
	}
	_, err := raw.normalize()
	if err != nil {
		t.Fatalf("unexpected error for empty start/done: %v", err)
	}
}

func TestNormalize_OverlaySkipsGraphValidation(t *testing.T) {
	// Overlays reference base nodes not available yet — skip validation.
	raw := rawCircuitDef{
		Circuit: "overlay-test",
		Import:  "base-circuit",
		Nodes: []rawNodeDef{
			{NodeDef: NodeDef{Name: "extra"}},
		},
		Edges: []EdgeDef{
			{ID: "extra-base_node", From: "extra", To: "base_node"},
		},
		Start: "base_start",
		Done:  "_done",
	}
	_, err := raw.normalize()
	if err != nil {
		t.Fatalf("overlay with base node refs should not fail: %v", err)
	}
}

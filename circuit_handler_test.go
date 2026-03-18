package framework

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuildGraph_CircuitHandler_NilRegistry(t *testing.T) {
	def := &CircuitDef{
		Circuit: "parent", Start: "main", Done: "done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "main", HandlerType: "circuit", Handler: "child"}},
		Edges:       []EdgeDef{{ID: "main-done", From: "main", To: "done"}},
	}
	_, err := def.BuildGraph(GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": &passthroughTransformer{}},
	})
	if err == nil {
		t.Fatal("expected error for nil circuit registry")
	}
	if !strings.Contains(err.Error(), "circuit registry is nil") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildGraph_CircuitHandler_WithRegistry(t *testing.T) {
	child := &CircuitDef{
		Circuit: "child", Start: "s1", Done: "d",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "s1", HandlerType: "transformer", Handler: "passthrough"}},
		Edges:       []EdgeDef{{ID: "s1-d", From: "s1", To: "d"}},
	}
	parent := &CircuitDef{
		Circuit: "parent", Start: "main", Done: "done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "main", HandlerType: "circuit", Handler: "child"}},
		Edges:       []EdgeDef{{ID: "main-done", From: "main", To: "done"}},
	}
	g, err := parent.BuildGraph(GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": &passthroughTransformer{}},
		Circuits:     map[string]*CircuitDef{"child": child},
	})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	node, ok := g.NodeByName("main")
	if !ok {
		t.Fatal("node 'main' not found")
	}
	if _, ok := node.(*circuitRefNode); !ok {
		t.Errorf("node type = %T, want *circuitRefNode", node)
	}
}

func TestOverlayMerge_InheritsBaseStartDoneAndAddsOverlayNodes(t *testing.T) {
	base := []byte(`
circuit: base
start: recall
done: DONE
handler_type: transformer
nodes:
  - name: recall
    handler: passthrough
    description: "base recall node"
  - name: triage
    handler: passthrough
edges:
  - id: recall-triage
    from: recall
    to: triage
  - id: triage-done
    from: triage
    to: DONE
`)
	overlay := []byte(`
import: base
circuit: base
nodes:
  - name: extra
    handler_type: circuit
    handler: sub-circuit
    edges:
      - name: extra-done
        to: DONE
        when: "true"
edges:
  - id: triage-done
    from: triage
    to: extra
`)
	resolver := func(name string) ([]byte, error) {
		if name == "base" {
			return base, nil
		}
		return nil, nil
	}

	def, err := LoadCircuitWithOverlay(overlay, resolver)
	if err != nil {
		t.Fatalf("LoadCircuitWithOverlay: %v", err)
	}

	// Base fields inherited.
	if def.Start != "recall" {
		t.Errorf("start = %q, want recall", def.Start)
	}
	if def.Done != "DONE" {
		t.Errorf("done = %q, want DONE", def.Done)
	}

	// Base nodes present.
	nodeNames := make(map[string]bool)
	for _, n := range def.Nodes {
		nodeNames[n.Name] = true
	}
	if !nodeNames["recall"] {
		t.Error("missing base node 'recall'")
	}
	if !nodeNames["triage"] {
		t.Error("missing base node 'triage'")
	}

	// Overlay node added.
	if !nodeNames["extra"] {
		t.Error("missing overlay node 'extra'")
	}

	// Overlay edge replaces base edge (same ID).
	for _, e := range def.Edges {
		if e.ID == "triage-done" && e.To != "extra" {
			t.Errorf("edge triage-done.to = %q, want 'extra' (overlay should replace)", e.To)
		}
	}
}

func TestBatchWalk_WithSubCircuit(t *testing.T) {
	// Parent circuit: main (handler_type: circuit) → done
	// Sub-circuit: step1 (passthrough) → sub-done
	// Verifies that BatchWalk resolves and walks the sub-circuit.

	child := &CircuitDef{
		Circuit: "child", Start: "step1", Done: "sub-done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "step1", HandlerType: "transformer", Handler: "passthrough"}},
		Edges:       []EdgeDef{{ID: "step1-done", From: "step1", To: "sub-done"}},
	}
	parent := &CircuitDef{
		Circuit: "parent", Start: "main", Done: "done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "main", HandlerType: "circuit", Handler: "child"}},
		Edges:       []EdgeDef{{ID: "main-done", From: "main", To: "done"}},
	}

	cases := []BatchCase{
		{ID: "C1", Context: map[string]any{"input": "hello"}},
	}

	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def: parent,
		Shared: GraphRegistries{
			Transformers: TransformerRegistry{"passthrough": &passthroughTransformer{}},
			Circuits:     map[string]*CircuitDef{"child": child},
		},
		Cases:    cases,
		Parallel: 1,
	})

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Error != nil {
		t.Fatalf("case C1 error: %v", results[0].Error)
	}
	if len(results[0].Path) == 0 {
		t.Error("case C1 has empty path — circuit didn't walk")
	}
	t.Logf("C1 path: %v", results[0].Path)
}

func TestLoadSubCircuitsFromFS(t *testing.T) {
	baseCircuit := `
circuit: child
start: s1
done: d
handler_type: transformer
nodes:
  - name: s1
    handler: passthrough
edges:
  - id: s1-d
    from: s1
    to: d
`
	overlay := `
import: child
circuit: child
nodes:
  - name: s2
    handler: passthrough
    edges:
      - name: s2-d
        to: d
        when: "true"
`
	fsys := fstest.MapFS{
		"circuits/child.yaml": &fstest.MapFile{Data: []byte(overlay)},
	}

	resolvers := map[string]AssetResolver{
		"child": func(name string) ([]byte, error) {
			if name == "child" {
				return []byte(baseCircuit), nil
			}
			return nil, nil
		},
	}

	circuits := LoadSubCircuitsFromFS(fsys, resolvers)
	if circuits == nil {
		t.Fatal("expected non-nil circuits map")
	}
	def, ok := circuits["child"]
	if !ok {
		t.Fatal("missing 'child' circuit")
	}
	if def.Start != "s1" {
		t.Errorf("start = %q, want s1 (from base)", def.Start)
	}

	// Overlay node should be merged.
	nodeNames := make(map[string]bool)
	for _, n := range def.Nodes {
		nodeNames[n.Name] = true
	}
	if !nodeNames["s2"] {
		t.Error("missing overlay node 's2'")
	}
}

// Verify the utility signature compiles — the function is defined in subcircuit.go
var _ = LoadSubCircuitsFromFS
var _ fs.FS

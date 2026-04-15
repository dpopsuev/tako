package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/identity"
)

type buildTestNode struct {
	name string
}

func (n *buildTestNode) Name() string                      { return n.name }
func (n *buildTestNode) Approach() identity.Element { return "" }
func (n *buildTestNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

func TestResolveByInstrument_Transformer(t *testing.T) {
	stub := InstrumentFunc("test-transformer", func(_ context.Context, _ *InstrumentContext) (any, error) {
		return "result", nil
	})
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{"test-transformer": stub},
	}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Action: "test-transformer", Instrument: "transformer"}

	node, err := resolveByInstrument(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveByInstrument: %v", err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
	if node.Name() != "test-node" {
		t.Errorf("Name = %q, want test-node", node.Name())
	}
}

func TestResolveByInstrument_UnknownInstrument(t *testing.T) {
	reg := &GraphRegistries{}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Action: "test", Instrument: "unknown_type"}

	_, err := resolveByInstrument(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for unknown instrument")
	}
	if !errors.Is(err, ErrNode) {
		t.Errorf("want ErrNode, got %v", err)
	}
}

func TestResolveByInstrument_NoInstrument(t *testing.T) {
	reg := &GraphRegistries{}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Action: "test"}

	_, err := resolveByInstrument(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for missing instrument")
	}
}

func TestResolveByInstrument_Node(t *testing.T) {
	reg := &GraphRegistries{
		Nodes: NodeRegistry{
			"custom": func(nd circuit.NodeDef) circuit.Node {
				return &buildTestNode{name: string(nd.Name)}
			},
		},
	}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Action: "custom", Instrument: "node"}

	node, err := resolveByInstrument(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveByInstrument: %v", err)
	}
	if node.Name() != "test-node" {
		t.Errorf("Name = %q", node.Name())
	}
}

func TestResolveByInstrument_NodeRegistryNil(t *testing.T) {
	reg := &GraphRegistries{} // Nodes is nil
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Action: "custom", Instrument: "node"}

	_, err := resolveByInstrument(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for nil node registry")
	}
}

func TestBuildGraph_MinimalCircuit(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Start:   "a",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "echo", Instrument: "transformer"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "a", To: "_done", When: "true"},
		},
	}
	stub := InstrumentFunc("echo", func(_ context.Context, _ *InstrumentContext) (any, error) {
		return "echo", nil
	})
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{"echo": stub},
	}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestBuildGraph_MissingTransformer(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Start:   "a",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "nonexistent", Instrument: "transformer"},
		},
	}
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{},
	}

	_, err := BuildGraph(def, reg)
	if err == nil {
		t.Fatal("expected error for missing transformer")
	}
}

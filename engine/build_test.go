package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

type buildTestNode struct {
	name string
}

func (n *buildTestNode) Name() string                    { return n.name }
func (n *buildTestNode) ElementAffinity() roster.Element { return "" }
func (n *buildTestNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

func TestResolveHandler_TransformerType(t *testing.T) {
	stub := TransformerFunc("test-transformer", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "result", nil
	})
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"test-transformer": stub},
	}
	def := &circuit.CircuitDef{HandlerType: circuit.HandlerTypeTransformer}
	nd := &circuit.NodeDef{Name: "test-node", Handler: "test-transformer"}

	node, err := resolveHandler(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveHandler: %v", err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
	if node.Name() != "test-node" {
		t.Errorf("Name = %q, want test-node", node.Name())
	}
}

func TestResolveHandler_UnknownType(t *testing.T) {
	reg := &GraphRegistries{}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Handler: "test", HandlerType: "unknown_type"}

	_, err := resolveHandler(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for unknown handler type")
	}
	if !errors.Is(err, ErrNode) {
		t.Errorf("want ErrNode, got %v", err)
	}
}

func TestResolveHandler_NoHandlerType(t *testing.T) {
	reg := &GraphRegistries{}
	def := &circuit.CircuitDef{} // no default handler_type
	nd := &circuit.NodeDef{Name: "test-node", Handler: "test"}

	_, err := resolveHandler(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for missing handler_type")
	}
}

func TestResolveHandler_FallsBackToCircuitHandlerType(t *testing.T) {
	stub := TransformerFunc("fallback", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "result", nil
	})
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"fallback": stub},
	}
	def := &circuit.CircuitDef{HandlerType: circuit.HandlerTypeTransformer}
	nd := &circuit.NodeDef{Name: "test-node", Handler: "fallback"} // no node-level handler_type

	node, err := resolveHandler(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveHandler: %v", err)
	}
	if node == nil {
		t.Fatal("expected node from circuit-level handler_type fallback")
	}
}

func TestResolveHandler_NodeType(t *testing.T) {
	reg := &GraphRegistries{
		Nodes: NodeRegistry{
			"custom": func(nd circuit.NodeDef) circuit.Node {
				return &buildTestNode{name: string(nd.Name)}
			},
		},
	}
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Handler: "custom", HandlerType: circuit.HandlerTypeNode}

	node, err := resolveHandler(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveHandler: %v", err)
	}
	if node.Name() != "test-node" {
		t.Errorf("Name = %q", node.Name())
	}
}

func TestResolveHandler_NodeRegistryNil(t *testing.T) {
	reg := &GraphRegistries{} // Nodes is nil
	def := &circuit.CircuitDef{}
	nd := &circuit.NodeDef{Name: "test-node", Handler: "custom", HandlerType: circuit.HandlerTypeNode}

	_, err := resolveHandler(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for nil node registry")
	}
}

func TestBuildGraph_MinimalCircuit(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Start:       "a",
		Done:        "_done",
		HandlerType: circuit.HandlerTypeTransformer,
		Nodes: []circuit.NodeDef{
			{Name: "a", Handler: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "a", To: "_done", When: "true"},
		},
	}
	stub := TransformerFunc("echo", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "echo", nil
	})
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"echo": stub},
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
		Circuit:     "test",
		Start:       "a",
		HandlerType: circuit.HandlerTypeTransformer,
		Nodes: []circuit.NodeDef{
			{Name: "a", Handler: "nonexistent"},
		},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{},
	}

	_, err := BuildGraph(def, reg)
	if err == nil {
		t.Fatal("expected error for missing transformer")
	}
}

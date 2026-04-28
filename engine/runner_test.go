package engine

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle/visual"
)

type runnerTestArtifact struct {
	typ  string
	conf float64
	raw  any
}

func (a *runnerTestArtifact) Type() string        { return a.typ }
func (a *runnerTestArtifact) Confidence() float64 { return a.conf }
func (a *runnerTestArtifact) Raw() any            { return a.raw }

type runnerTestNode struct {
	name    string
	element visual.Element
	out     circuit.Artifact
	err     error
}

func (n *runnerTestNode) Name() string               { return n.name }
func (n *runnerTestNode) Approach() visual.Element { return n.element }
func (n *runnerTestNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return n.out, n.err
}

type runnerTestWalker struct {
	identity circuit.AgentIdentity
	state    *circuit.WalkerState
	visited  []string
}

func (w *runnerTestWalker) Identity() circuit.AgentIdentity       { return w.identity }
func (w *runnerTestWalker) SetIdentity(id *circuit.AgentIdentity) { w.identity = *id }
func (w *runnerTestWalker) State() *circuit.WalkerState        { return w.state }
func (w *runnerTestWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	w.visited = append(w.visited, node.Name())
	return node.Process(ctx, nc)
}

func TestRunner_Walk_NoSchema(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "no-schema",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "stub", Instrument: "node"},
			{Name: "b", Action: "stub", Instrument: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"x": 1}}
	nodeReg := NodeRegistry{
		"stub": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(walker.visited) != 2 {
		t.Errorf("visited %d nodes, want 2", len(walker.visited))
	}
}

func TestRunner_Walk_SchemaPass(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "schema-pass",
		Nodes: []circuit.NodeDef{
			{
				Name:       "a",
				Instrument: "node",
				Action:     "stub",
				Schema: &circuit.ArtifactSchema{
					Type:     "object",
					Required: []string{"id"},
					Fields:   map[string]circuit.FieldSchema{"id": {Type: "string"}},
				},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"id": "C1"}}
	nodeReg := NodeRegistry{
		"stub": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.Logger = slog.Default()

	walker := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk should pass schema: %v", err)
	}
}

func TestRunner_Walk_SchemaFail(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "schema-fail",
		Nodes: []circuit.NodeDef{
			{
				Name:       "a",
				Instrument: "node",
				Action:     "stub",
				Schema: &circuit.ArtifactSchema{
					Type:     "object",
					Required: []string{"score"},
					Fields:   map[string]circuit.FieldSchema{"score": {Type: "number"}},
				},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"wrong": "data"}}
	nodeReg := NodeRegistry{
		"stub": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	err = runner.Walk(context.Background(), walker, "a")
	if err == nil {
		t.Fatal("Walk should fail schema validation")
	}
}

func TestRunner_Walk_NodeError(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "node-error",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "failing", Instrument: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	nodeReg := NodeRegistry{
		"failing": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), err: fmt.Errorf("node exploded")}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	err = runner.Walk(context.Background(), walker, "a")
	if err == nil {
		t.Fatal("Walk should propagate node error")
	}
}

func TestRunner_Walk_MultiNodeWithSchema(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "multi-schema",
		Nodes: []circuit.NodeDef{
			{
				Name:       "a",
				Instrument: "node",
				Action:     "stub",
				Schema: &circuit.ArtifactSchema{
					Type:     "object",
					Required: []string{"name"},
					Fields:   map[string]circuit.FieldSchema{"name": {Type: "string"}},
				},
			},
			{
				Name:       "b",
				Instrument: "node",
				Action:     "stub",
			},
			{
				Name:       "c",
				Instrument: "node",
				Action:     "stub",
				Schema: &circuit.ArtifactSchema{
					Type: "object",
					Fields: map[string]circuit.FieldSchema{
						"count": {Type: "number"},
					},
				},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "c", Name: "b-c"},
			{ID: "E3", From: "c", To: "_done", Name: "c-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{
		"name": "test", "count": float64(5),
	}}
	nodeReg := NodeRegistry{
		"stub": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(walker.visited) != 3 {
		t.Errorf("visited %d nodes, want 3", len(walker.visited))
	}
}

func TestNewRunner_InvalidCircuit(t *testing.T) {
	def := &circuit.CircuitDef{Circuit: ""}
	_, err := NewRunner(def, NodeRegistry{}, EdgeFactory{})
	if err == nil {
		t.Fatal("NewRunner should fail for invalid circuit")
	}
}

func TestRunner_Walk_NilWalker(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "nil-walker",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Instrument: "transformer", Action: "echo"},
			{Name: "b", Approach: "analytical", Instrument: "transformer", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b", When: "true"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{"echo": &echoTransformer{}},
	}
	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	if err := runner.Walk(context.Background(), nil, "a"); err != nil {
		t.Fatalf("Walk with nil walker: %v", err)
	}
}

func TestProcessWalker(t *testing.T) {
	pw := circuit.NewProcessWalker("test-id")
	if pw.Identity().Name != "test-id" {
		t.Errorf("Name = %q, want test-id", pw.Identity().Name)
	}
	if pw.State().ID != "test-id" {
		t.Errorf("State.ID = %q, want test-id", pw.State().ID)
	}

	node := &runnerTestNode{
		name: "n",
		out:  &runnerTestArtifact{typ: "t", conf: 1.0, raw: "data"},
	}
	art, err := pw.Handle(context.Background(), node, circuit.NodeContext{})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if art.Raw() != "data" {
		t.Errorf("Raw() = %v, want data", art.Raw())
	}
}

func TestRunner_SchemasExtracted(t *testing.T) {
	schema := &circuit.ArtifactSchema{Type: "object", Required: []string{"id"}}
	def := &circuit.CircuitDef{
		Circuit: "schemas",
		Nodes: []circuit.NodeDef{
			{Name: "a", Action: "stub", Instrument: "node", Schema: schema},
			{Name: "b", Action: "stub", Instrument: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{raw: map[string]any{"id": "x"}}
	nodeReg := NodeRegistry{
		"stub": func(d circuit.NodeDef) circuit.Node {
			return &runnerTestNode{name: string(d.Name), out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	if runner.Schemas["a"] == nil {
		t.Error("schema for node 'a' not extracted")
	}
	if runner.Schemas["b"] != nil {
		t.Error("schema for node 'b' should be nil")
	}
}

package framework

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
)

type runnerTestArtifact struct {
	typ  string
	conf float64
	raw  any
}

func (a *runnerTestArtifact) Type() string       { return a.typ }
func (a *runnerTestArtifact) Confidence() float64 { return a.conf }
func (a *runnerTestArtifact) Raw() any            { return a.raw }

type runnerTestNode struct {
	name    string
	element Element
	out     Artifact
	err     error
}

func (n *runnerTestNode) Name() string            { return n.name }
func (n *runnerTestNode) ElementAffinity() Element { return n.element }
func (n *runnerTestNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return n.out, n.err
}

type runnerTestWalker struct {
	identity AgentIdentity
	state    *WalkerState
	visited  []string
}

func (w *runnerTestWalker) Identity() AgentIdentity      { return w.identity }
func (w *runnerTestWalker) SetIdentity(id AgentIdentity)  { w.identity = id }
func (w *runnerTestWalker) State() *WalkerState           { return w.state }
func (w *runnerTestWalker) Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error) {
	w.visited = append(w.visited, node.Name())
	return node.Process(ctx, nc)
}

func TestRunner_Walk_NoSchema(t *testing.T) {
	def := &CircuitDef{
		Circuit: "no-schema",
		Nodes: []NodeDef{
			{Name: "a", Handler: "stub", HandlerType: "node"},
			{Name: "b", Handler: "stub", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"x": 1}}
	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(walker.visited) != 2 {
		t.Errorf("visited %d nodes, want 2", len(walker.visited))
	}
}

func TestRunner_Walk_SchemaPass(t *testing.T) {
	def := &CircuitDef{
		Circuit: "schema-pass",
		Nodes: []NodeDef{
			{
				Name:   "a",
				Handler: "stub", HandlerType: "node",
				Schema: &ArtifactSchema{
					Type:     "object",
					Required: []string{"id"},
					Fields:   map[string]FieldSchema{"id": {Type: "string"}},
				},
			},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"id": "C1"}}
	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.Logger = slog.Default()

	walker := &runnerTestWalker{state: NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk should pass schema: %v", err)
	}
}

func TestRunner_Walk_SchemaFail(t *testing.T) {
	def := &CircuitDef{
		Circuit: "schema-fail",
		Nodes: []NodeDef{
			{
				Name:   "a",
				Handler: "stub", HandlerType: "node",
				Schema: &ArtifactSchema{
					Type:     "object",
					Required: []string{"score"},
					Fields:   map[string]FieldSchema{"score": {Type: "number"}},
				},
			},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{typ: "test", conf: 1.0, raw: map[string]any{"wrong": "data"}}
	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: NewWalkerState("test")}
	err = runner.Walk(context.Background(), walker, "a")
	if err == nil {
		t.Fatal("Walk should fail schema validation")
	}
}

func TestRunner_Walk_NodeError(t *testing.T) {
	def := &CircuitDef{
		Circuit: "node-error",
		Nodes: []NodeDef{
			{Name: "a", Handler: "failing", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "_done", Name: "a-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	nodeReg := NodeRegistry{
		"failing": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, err: fmt.Errorf("node exploded")}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: NewWalkerState("test")}
	err = runner.Walk(context.Background(), walker, "a")
	if err == nil {
		t.Fatal("Walk should propagate node error")
	}
}

func TestRunner_Walk_MultiNodeWithSchema(t *testing.T) {
	def := &CircuitDef{
		Circuit: "multi-schema",
		Nodes: []NodeDef{
			{
				Name:   "a",
				Handler: "stub", HandlerType: "node",
				Schema: &ArtifactSchema{
					Type:     "object",
					Required: []string{"name"},
					Fields:   map[string]FieldSchema{"name": {Type: "string"}},
				},
			},
			{
				Name:   "b",
				Handler: "stub", HandlerType: "node",
			},
			{
				Name:   "c",
				Handler: "stub", HandlerType: "node",
				Schema: &ArtifactSchema{
					Type: "object",
					Fields: map[string]FieldSchema{
						"count": {Type: "number"},
					},
				},
			},
		},
		Edges: []EdgeDef{
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
		"stub": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, out: art}
		},
	}

	runner, err := NewRunner(def, nodeReg, EdgeFactory{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	walker := &runnerTestWalker{state: NewWalkerState("test")}
	if err := runner.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(walker.visited) != 3 {
		t.Errorf("visited %d nodes, want 3", len(walker.visited))
	}
}

func TestNewRunner_InvalidCircuit(t *testing.T) {
	def := &CircuitDef{Circuit: ""}
	_, err := NewRunner(def, NodeRegistry{}, EdgeFactory{})
	if err == nil {
		t.Fatal("NewRunner should fail for invalid circuit")
	}
}

func TestRunner_Walk_NilWalker(t *testing.T) {
	def := &CircuitDef{
		Circuit:     "nil-walker",
		HandlerType: "transformer",
		Nodes: []NodeDef{
			{Name: "a", Approach: "rapid", Handler: "echo"},
			{Name: "b", Approach: "analytical", Handler: "echo"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b", When: "true"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{"echo": &echoTransformer{}},
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
	pw := NewProcessWalker("test-id")
	if pw.Identity().PersonaName != "process" {
		t.Errorf("PersonaName = %q, want process", pw.Identity().PersonaName)
	}
	if pw.State().ID != "test-id" {
		t.Errorf("State.ID = %q, want test-id", pw.State().ID)
	}

	node := &runnerTestNode{
		name: "n",
		out:  &runnerTestArtifact{typ: "t", conf: 1.0, raw: "data"},
	}
	art, err := pw.Handle(context.Background(), node, NodeContext{})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if art.Raw() != "data" {
		t.Errorf("Raw() = %v, want data", art.Raw())
	}
}

func TestRunner_SchemasExtracted(t *testing.T) {
	schema := &ArtifactSchema{Type: "object", Required: []string{"id"}}
	def := &CircuitDef{
		Circuit: "schemas",
		Nodes: []NodeDef{
			{Name: "a", Handler: "stub", HandlerType: "node", Schema: schema},
			{Name: "b", Handler: "stub", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done"},
		},
		Start: "a",
		Done:  "_done",
	}

	art := &runnerTestArtifact{raw: map[string]any{"id": "x"}}
	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node {
			return &runnerTestNode{name: d.Name, out: art}
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

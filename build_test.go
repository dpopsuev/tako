package framework

import (
	"context"
	"os"
	"testing"
)

func TestBuildGraph_SimpleWalk(t *testing.T) {
	def := &CircuitDef{
		Circuit: "simple",
		Nodes: []NodeDef{
			{Name: "a", Handler: "stub", HandlerType: "node"},
			{Name: "b", Handler: "stub", HandlerType: "node"},
			{Name: "c", Handler: "stub", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", Name: "a-to-b", From: "a", To: "b"},
			{ID: "E2", Name: "b-to-c", From: "b", To: "c"},
			{ID: "E3", Name: "c-done", From: "c", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}

	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
	}
	edgeFactory := EdgeFactory{}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: nodeReg, Edges: edgeFactory})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	if graph.Name() != "simple" {
		t.Errorf("Name = %q, want %q", graph.Name(), "simple")
	}
	if len(graph.Nodes()) != 3 {
		t.Errorf("len(Nodes) = %d, want 3", len(graph.Nodes()))
	}
	if len(graph.Edges()) != 3 {
		t.Errorf("len(Edges) = %d, want 3", len(graph.Edges()))
	}

	walker := &stubBuildWalker{state: NewWalkerState("test-walker")}
	if err := graph.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if walker.state.Status != "done" {
		t.Errorf("Status = %q, want %q", walker.state.Status, "done")
	}
	if len(walker.state.History) != 3 {
		t.Errorf("len(History) = %d, want 3", len(walker.state.History))
	}
}

func TestBuildGraph_WithZones(t *testing.T) {
	def := &CircuitDef{
		Circuit: "zoned",
		Nodes: []NodeDef{
			{Name: "a", Handler: "stub", HandlerType: "node"},
			{Name: "b", Handler: "stub", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "a-b"},
			{ID: "E2", From: "b", To: "_done", Name: "b-done"},
		},
		Zones: map[string]ZoneDef{
			"front": {Nodes: []string{"a"}, Approach: "rapid", Stickiness: 1},
			"back":  {Nodes: []string{"b"}, Approach: "analytical"},
		},
		Start: "a",
		Done:  "_done",
	}

	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: nodeReg, Edges: EdgeFactory{}})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(graph.Zones()) != 2 {
		t.Errorf("len(Zones) = %d, want 2", len(graph.Zones()))
	}
}

func TestBuildGraph_CustomEdgeFactory(t *testing.T) {
	def := &CircuitDef{
		Circuit: "custom-edges",
		Nodes: []NodeDef{
			{Name: "a", Handler: "stub", HandlerType: "node"},
			{Name: "b", Handler: "stub", HandlerType: "node"},
		},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b", Name: "custom"},
			{ID: "E2", From: "b", To: "_done", Name: "done"},
		},
		Start: "a",
		Done:  "_done",
	}

	customCalled := false
	edgeFactory := EdgeFactory{
		"E1": func(d EdgeDef) Edge {
			return &stubBuildEdge{
				id: d.ID, from: d.From, to: d.To,
				evaluate: func(a Artifact, s *WalkerState) *Transition {
					customCalled = true
					return &Transition{NextNode: d.To, Explanation: "custom logic"}
				},
			}
		},
	}

	nodeReg := NodeRegistry{
		"stub": func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: nodeReg, Edges: edgeFactory})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := &stubBuildWalker{state: NewWalkerState("test")}
	if err := graph.Walk(context.Background(), walker, "a"); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !customCalled {
		t.Error("custom edge factory was not invoked")
	}
}

func TestBuildGraph_MissingNodeFactory(t *testing.T) {
	def := &CircuitDef{
		Circuit: "missing",
		Nodes:    []NodeDef{{Name: "a", Handler: "nonexistent", HandlerType: "node"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	_, err := BuildGraph(def, GraphRegistries{Nodes: NodeRegistry{}, Edges: EdgeFactory{}})
	if err == nil {
		t.Fatal("expected error for missing node factory")
	}
}

func TestBuildGraph_RealF0F6_Structure(t *testing.T) {
	data, err := os.ReadFile("testdata/rca-investigation.yaml")
	if err != nil {
		t.Fatalf("read YAML: %v", err)
	}
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	nodeReg := NodeRegistry{
		"recall":      func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"triage":      func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"resolve":     func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"investigate": func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"correlate":   func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"review":      func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
		"report":      func(d NodeDef) Node { return &stubBuildNode{name: d.Name} },
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: nodeReg, Edges: EdgeFactory{}})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(graph.Nodes()) != 7 {
		t.Errorf("len(Nodes) = %d, want 7", len(graph.Nodes()))
	}
	if len(graph.Zones()) != 3 {
		t.Errorf("len(Zones) = %d, want 3", len(graph.Zones()))
	}

	// Verify specific node lookup
	recall, ok := graph.NodeByName("recall")
	if !ok {
		t.Fatal("recall node not found")
	}
	if recall.Name() != "recall" {
		t.Errorf("recall.Name() = %q", recall.Name())
	}

	recallEdges := graph.EdgesFrom("recall")
	if len(recallEdges) != 3 {
		t.Errorf("EdgesFrom(recall) = %d edges, want 3", len(recallEdges))
	}
}

// --- stubs for build tests ---

type stubBuildNode struct {
	name    string
	element Element
}

func (n *stubBuildNode) Name() string              { return n.name }
func (n *stubBuildNode) ElementAffinity() Element   { return n.element }
func (n *stubBuildNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return &stubBuildArtifact{}, nil
}

type stubBuildArtifact struct{}

func (a *stubBuildArtifact) Type() string       { return "stub" }
func (a *stubBuildArtifact) Confidence() float64 { return 1.0 }
func (a *stubBuildArtifact) Raw() any            { return nil }

type stubBuildWalker struct {
	identity AgentIdentity
	state    *WalkerState
}

func (w *stubBuildWalker) Identity() AgentIdentity      { return w.identity }
func (w *stubBuildWalker) SetIdentity(id AgentIdentity)  { w.identity = id }
func (w *stubBuildWalker) State() *WalkerState           { return w.state }
func (w *stubBuildWalker) Handle(_ context.Context, node Node, nc NodeContext) (Artifact, error) {
	return node.Process(context.Background(), nc)
}

type stubBuildEdge struct {
	id       string
	from     string
	to       string
	evaluate func(Artifact, *WalkerState) *Transition
}

func (e *stubBuildEdge) ID() string       { return e.id }
func (e *stubBuildEdge) From() string     { return e.from }
func (e *stubBuildEdge) To() string       { return e.to }
func (e *stubBuildEdge) IsShortcut() bool { return false }
func (e *stubBuildEdge) IsLoop() bool     { return false }
func (e *stubBuildEdge) Evaluate(a Artifact, s *WalkerState) *Transition {
	if e.evaluate != nil {
		return e.evaluate(a, s)
	}
	return &Transition{NextNode: e.to}
}

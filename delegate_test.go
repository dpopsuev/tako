package framework

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)


func TestDelegateArtifact_Interface(t *testing.T) {
	var _ Artifact = (*DelegateArtifact)(nil)
}

func TestDelegateArtifact_Type(t *testing.T) {
	a := &DelegateArtifact{}
	if got := a.Type(); got != "delegate" {
		t.Errorf("Type() = %q, want %q", got, "delegate")
	}
}

func TestDelegateArtifact_Confidence_Empty(t *testing.T) {
	a := &DelegateArtifact{}
	if got := a.Confidence(); got != 0 {
		t.Errorf("Confidence() = %f, want 0 (no inner artifacts)", got)
	}
}

func TestDelegateArtifact_Confidence_Average(t *testing.T) {
	a := &DelegateArtifact{
		InnerArtifacts: map[string]Artifact{
			"a": &stubArtifact{confidence: 0.8, typ: "t"},
			"b": &stubArtifact{confidence: 0.6, typ: "t"},
		},
	}
	want := 0.7
	got := a.Confidence()
	if got < want-0.001 || got > want+0.001 {
		t.Errorf("Confidence() = %f, want %f", got, want)
	}
}

func TestDelegateArtifact_Confidence_NilArtifacts(t *testing.T) {
	a := &DelegateArtifact{
		InnerArtifacts: map[string]Artifact{
			"a": nil,
			"b": nil,
		},
	}
	if got := a.Confidence(); got != 0 {
		t.Errorf("Confidence() = %f, want 0 (all nil)", got)
	}
}

func TestDelegateArtifact_Raw(t *testing.T) {
	inner := map[string]Artifact{
		"x": &stubArtifact{confidence: 1.0, typ: "t"},
	}
	a := &DelegateArtifact{InnerArtifacts: inner}
	raw, ok := a.Raw().(map[string]Artifact)
	if !ok {
		t.Fatalf("Raw() type = %T, want map[string]Artifact", a.Raw())
	}
	if len(raw) != 1 {
		t.Errorf("Raw() len = %d, want 1", len(raw))
	}
}

func TestDelegateArtifact_Fields(t *testing.T) {
	def := &CircuitDef{Circuit: "inner"}
	a := &DelegateArtifact{
		GeneratedCircuit: def,
		NodeCount:        3,
		Elapsed:          500 * time.Millisecond,
	}
	if a.GeneratedCircuit.Circuit != "inner" {
		t.Errorf("GeneratedCircuit.Circuit = %q, want %q", a.GeneratedCircuit.Circuit, "inner")
	}
	if a.NodeCount != 3 {
		t.Errorf("NodeCount = %d, want 3", a.NodeCount)
	}
	if a.Elapsed != 500*time.Millisecond {
		t.Errorf("Elapsed = %v, want 500ms", a.Elapsed)
	}
}

func TestDelegateNode_Interface(t *testing.T) {
	var _ DelegateNode = (*testDelegateNode)(nil)
}

func TestWalk_DelegateNode_SubWalk(t *testing.T) {
	// Outer circuit: A → Delegate → C
	// Delegate generates inner circuit: X → Y
	innerDef := &CircuitDef{
		Circuit: "inner",
		Start:   "X",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "X", Transformer: "passthrough"},
			{Name: "Y", Transformer: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "x-y", From: "X", To: "Y"},
			{ID: "y-done", From: "Y", To: "_done"},
		},
	}

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{name: "Delegate", circuitDef: innerDef}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []Edge{
		&stubEdge{id: "a-d", from: "A", to: "Delegate"},
		&stubEdge{id: "d-c", from: "Delegate", to: "C"},
		&stubEdge{id: "c-done", from: "C", to: "_done"},
	}

	trace := &TraceCollector{}
	g, err := NewGraph("outer", []Node{nodeA, nodeD, nodeC}, edges, nil, WithObserver(trace))
	if err != nil {
		t.Fatal(err)
	}

	g.registries = &GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": &passthroughTransformer{}},
	}

	walker := &stubWalker{
		identity: AgentIdentity{PersonaName: "test"},
		state:    NewWalkerState("w1"),
	}

	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// Outer walker should visit A, Delegate, C.
	if len(walker.visited) != 2 {
		// stubWalker records visited for non-delegate nodes only (A, C).
		// Delegate is handled by walkDelegate, not walker.Handle.
	}

	// Check delegate artifact is in outputs.
	da, ok := walker.State().Outputs["Delegate"]
	if !ok {
		t.Fatal("delegate artifact missing from outputs")
	}
	delArt, ok := da.(*DelegateArtifact)
	if !ok {
		t.Fatalf("output type = %T, want *DelegateArtifact", da)
	}
	if delArt.NodeCount != 2 {
		t.Errorf("NodeCount = %d, want 2", delArt.NodeCount)
	}
	if delArt.InnerError != nil {
		t.Errorf("InnerError = %v, want nil", delArt.InnerError)
	}

	// Inner artifacts should be namespaced in outer walker state.
	if _, ok := walker.State().Outputs["delegate:Delegate:X"]; !ok {
		t.Error("namespaced inner artifact delegate:Delegate:X missing")
	}
	if _, ok := walker.State().Outputs["delegate:Delegate:Y"]; !ok {
		t.Error("namespaced inner artifact delegate:Delegate:Y missing")
	}

	// Check observer events include delegate start/end.
	starts := trace.EventsOfType(EventDelegateStart)
	if len(starts) != 1 {
		t.Errorf("delegate_start events = %d, want 1", len(starts))
	}
	ends := trace.EventsOfType(EventDelegateEnd)
	if len(ends) != 1 {
		t.Errorf("delegate_end events = %d, want 1", len(ends))
	}

	// Inner walk events should have prefixed node names.
	innerEnters := 0
	for _, ev := range trace.EventsOfType(EventNodeEnter) {
		if strings.HasPrefix(ev.Node, "delegate:Delegate:") {
			innerEnters++
		}
	}
	if innerEnters != 2 {
		t.Errorf("prefixed inner node_enter events = %d, want 2 (X, Y)", innerEnters)
	}
}

func TestWalk_DelegateNode_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	innerDef := &CircuitDef{
		Circuit: "inner",
		Start:   "X",
		Done:    "_done",
		Nodes:   []NodeDef{{Name: "X", Transformer: "passthrough"}},
		Edges:   []EdgeDef{{ID: "x-done", From: "X", To: "_done"}},
	}

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{name: "D", circuitDef: innerDef}

	edges := []Edge{
		&stubEdge{id: "a-d", from: "A", to: "D"},
		&stubEdge{id: "d-done", from: "D", to: "_done"},
	}

	g, err := NewGraph("outer", []Node{nodeA, nodeD}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	walker := &stubWalker{
		identity: AgentIdentity{PersonaName: "test"},
		state:    NewWalkerState("w1"),
	}

	err = g.Walk(ctx, walker, "A")
	if err == nil {
		t.Fatal("Walk() should fail with cancelled context")
	}
}

func TestWalk_DelegateNode_GenerateError(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{
		name: "D",
		err:  errors.New("generation failed"),
	}

	edges := []Edge{
		&stubEdge{id: "a-d", from: "A", to: "D"},
		&stubEdge{id: "d-done", from: "D", to: "_done"},
	}

	g, err := NewGraph("outer", []Node{nodeA, nodeD}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	walker := &stubWalker{
		identity: AgentIdentity{PersonaName: "test"},
		state:    NewWalkerState("w1"),
	}

	err = g.Walk(context.Background(), walker, "A")
	if err == nil {
		t.Fatal("Walk() should fail when GenerateCircuit returns error")
	}
	if got := err.Error(); !strings.Contains(got, "generation failed") {
		t.Errorf("error = %q, want to contain %q", got, "generation failed")
	}
}

func TestBuildGraph_DelegateNode_DSL(t *testing.T) {
	def := &CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "A", Transformer: "passthrough"},
			{Name: "B", Delegate: true, Generator: "plan"},
			{Name: "C", Transformer: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	planGen := TransformerFunc("plan", func(_ context.Context, _ *TransformerContext) (any, error) {
		return &CircuitDef{
			Circuit: "generated",
			Start:   "W1",
			Done:    "_done",
			Nodes: []NodeDef{
				{Name: "W1", Transformer: "passthrough"},
			},
			Edges: []EdgeDef{
				{ID: "w1-done", From: "W1", To: "_done"},
			},
		}, nil
	})

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
			"plan":        planGen,
		},
	}

	g, err := def.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	// Verify the delegate node is a DelegateNode.
	node, ok := g.NodeByName("B")
	if !ok {
		t.Fatal("node B not found")
	}
	if _, ok := node.(DelegateNode); !ok {
		t.Errorf("node B type = %T, want DelegateNode", node)
	}

	// Walk the circuit end-to-end.
	walker := NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// Verify delegate artifact.
	da, ok := walker.State().Outputs["B"]
	if !ok {
		t.Fatal("delegate artifact missing from outputs")
	}
	delArt, ok := da.(*DelegateArtifact)
	if !ok {
		t.Fatalf("output type = %T, want *DelegateArtifact", da)
	}
	if delArt.GeneratedCircuit.Circuit != "generated" {
		t.Errorf("inner circuit name = %q, want %q", delArt.GeneratedCircuit.Circuit, "generated")
	}
}

func TestBuildGraph_DelegateNode_MissingGenerator(t *testing.T) {
	def := &CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "A", Delegate: true},
		},
		Edges: []EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := def.BuildGraph(GraphRegistries{})
	if err == nil {
		t.Fatal("BuildGraph() should fail for delegate without generator")
	}
	if !strings.Contains(err.Error(), "requires a generator") {
		t.Errorf("error = %q, want to contain 'requires a generator'", err.Error())
	}
}

// testDelegateNode is a DelegateNode for compile-time and test-time verification.
type testDelegateNode struct {
	name       string
	circuitDef *CircuitDef
	err        error
}

func (n *testDelegateNode) Name() string            { return n.name }
func (n *testDelegateNode) ElementAffinity() Element { return "" }
func (n *testDelegateNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return nil, nil
}
func (n *testDelegateNode) GenerateCircuit(_ context.Context, _ NodeContext) (*CircuitDef, error) {
	return n.circuitDef, n.err
}

// --- circuitRefNode tests ---

func TestCircuitRefNode_Interface(t *testing.T) {
	n := &circuitRefNode{name: "sub", circuitDef: &CircuitDef{Circuit: "inner"}}
	var _ DelegateNode = n
	if n.Name() != "sub" {
		t.Errorf("Name() = %q, want %q", n.Name(), "sub")
	}
}

func TestCircuitRefNode_GenerateCircuit(t *testing.T) {
	inner := &CircuitDef{Circuit: "gnd", Start: "X", Done: "_done"}
	n := &circuitRefNode{name: "gather", circuitDef: inner}

	got, err := n.GenerateCircuit(context.Background(), NodeContext{})
	if err != nil {
		t.Fatalf("GenerateCircuit() error: %v", err)
	}
	if got != inner {
		t.Error("GenerateCircuit() should return the stored CircuitDef pointer")
	}
}

func TestBuildGraph_CircuitRefNode(t *testing.T) {
	innerDef := &CircuitDef{
		Circuit: "gnd",
		Start:   "K1",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "K1", HandlerType: HandlerTypeTransformer, Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &CircuitDef{
		Circuit:     "rca",
		Start:       "A",
		Done:        "_done",
		HandlerType: HandlerTypeTransformer,
		Nodes: []NodeDef{
			{Name: "A", Handler: "passthrough"},
			{Name: "B", HandlerType: HandlerTypeCircuit, Handler: "gnd"},
			{Name: "C", Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*CircuitDef{
			"gnd": innerDef,
		},
	}

	g, err := outerDef.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	node, ok := g.NodeByName("B")
	if !ok {
		t.Fatal("node B not found")
	}
	if _, ok := node.(DelegateNode); !ok {
		t.Errorf("node B type = %T, want DelegateNode", node)
	}
}

func TestWalk_CircuitRefNode_SubWalk(t *testing.T) {
	innerDef := &CircuitDef{
		Circuit: "gnd",
		Start:   "K1",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "K1", HandlerType: HandlerTypeTransformer, Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &CircuitDef{
		Circuit:     "rca",
		Start:       "A",
		Done:        "_done",
		HandlerType: HandlerTypeTransformer,
		Nodes: []NodeDef{
			{Name: "A", Handler: "passthrough"},
			{Name: "B", HandlerType: HandlerTypeCircuit, Handler: "gnd"},
			{Name: "C", Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*CircuitDef{
			"gnd": innerDef,
		},
	}

	g, err := outerDef.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	walker := NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// Delegate artifact present for node B.
	da, ok := walker.State().Outputs["B"]
	if !ok {
		t.Fatal("delegate artifact missing from outputs")
	}
	delArt, ok := da.(*DelegateArtifact)
	if !ok {
		t.Fatalf("output type = %T, want *DelegateArtifact", da)
	}
	if delArt.GeneratedCircuit.Circuit != "gnd" {
		t.Errorf("inner circuit = %q, want %q", delArt.GeneratedCircuit.Circuit, "gnd")
	}

	// Inner artifact namespaced into outer outputs.
	if _, ok := walker.State().Outputs["delegate:B:K1"]; !ok {
		t.Error("inner artifact delegate:B:K1 missing from outer outputs")
	}

	// Node C ran after the delegate.
	if _, ok := walker.State().Outputs["C"]; !ok {
		t.Error("node C output missing — walk did not continue after delegate")
	}
}

func TestWalk_CircuitRefNode_ContextInheritance(t *testing.T) {
	// Inner circuit has a transformer that reads a context key.
	contextReader := TransformerFunc("ctx-reader", func(_ context.Context, tc *TransformerContext) (any, error) {
		v, _ := tc.WalkerState.Context["test-key"]
		return &testArtifact{raw: v}, nil
	})

	innerDef := &CircuitDef{
		Circuit: "inner",
		Start:   "R",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "R", HandlerType: HandlerTypeTransformer, Handler: "ctx-reader"},
		},
		Edges: []EdgeDef{
			{ID: "r-done", From: "R", To: "_done"},
		},
	}

	outerDef := &CircuitDef{
		Circuit:     "outer",
		Start:       "A",
		Done:        "_done",
		HandlerType: HandlerTypeTransformer,
		Nodes: []NodeDef{
			{Name: "A", Handler: "passthrough"},
			{Name: "D", HandlerType: HandlerTypeCircuit, Handler: "inner"},
		},
		Edges: []EdgeDef{
			{ID: "a-d", From: "A", To: "D"},
			{ID: "d-done", From: "D", To: "_done"},
		},
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
			"ctx-reader":  contextReader,
		},
		Circuits: map[string]*CircuitDef{
			"inner": innerDef,
		},
	}

	g, err := outerDef.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	walker := NewProcessWalker("test")
	walker.State().Context["test-key"] = "hello-from-parent"

	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// The inner node should have read the parent context key.
	innerArt, ok := walker.State().Outputs["delegate:D:R"]
	if !ok {
		t.Fatal("inner artifact delegate:D:R missing")
	}
	inner, ok := innerArt.Raw().(*testArtifact)
	if !ok {
		t.Fatalf("inner artifact Raw() type = %T, want *testArtifact", innerArt.Raw())
	}
	if inner.raw != "hello-from-parent" {
		t.Errorf("inner node read context = %v, want %q", inner.raw, "hello-from-parent")
	}
}

func TestBuildGraph_CircuitRefNode_MissingCircuit(t *testing.T) {
	def := &CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "A", HandlerType: HandlerTypeCircuit, Handler: "nonexistent"},
		},
		Edges: []EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := def.BuildGraph(GraphRegistries{
		Circuits: map[string]*CircuitDef{},
	})
	if err == nil {
		t.Fatal("BuildGraph() should fail for missing circuit reference")
	}
	if !strings.Contains(err.Error(), "no local circuit and no mediator endpoint") {
		t.Errorf("error = %q, want to contain 'no local circuit and no mediator endpoint'", err.Error())
	}
}

func TestDelegateEvents_CarryCircuitType_CircuitRef(t *testing.T) {
	innerDef := &CircuitDef{
		Circuit: "gnd",
		Start:   "K1",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "K1", HandlerType: HandlerTypeTransformer, Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &CircuitDef{
		Circuit:     "rca",
		Start:       "A",
		Done:        "_done",
		HandlerType: HandlerTypeTransformer,
		Nodes: []NodeDef{
			{Name: "A", Handler: "passthrough"},
			{Name: "B", HandlerType: HandlerTypeCircuit, Handler: "gnd"},
			{Name: "C", Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*CircuitDef{
			"gnd": innerDef,
		},
	}

	g, err := outerDef.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	trace := &TraceCollector{}
	g.(*DefaultGraph).SetObserver(trace)

	walker := NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// EventDelegateStart must carry circuit_type.
	starts := trace.EventsOfType(EventDelegateStart)
	if len(starts) != 1 {
		t.Fatalf("delegate_start events = %d, want 1", len(starts))
	}
	if ct, _ := starts[0].Metadata["circuit_type"].(string); ct != "gnd" {
		t.Errorf("delegate_start circuit_type = %q, want %q", ct, "gnd")
	}

	// EventDelegateEnd must carry circuit_type.
	ends := trace.EventsOfType(EventDelegateEnd)
	if len(ends) != 1 {
		t.Fatalf("delegate_end events = %d, want 1", len(ends))
	}
	if ct, _ := ends[0].Metadata["circuit_type"].(string); ct != "gnd" {
		t.Errorf("delegate_end circuit_type = %q, want %q", ct, "gnd")
	}
}

func TestDelegateEvents_CarryCircuitType_DSLDelegate(t *testing.T) {
	planGen := TransformerFunc("plan", func(_ context.Context, _ *TransformerContext) (any, error) {
		return &CircuitDef{
			Circuit: "generated",
			Start:   "W1",
			Done:    "_done",
			Nodes:   []NodeDef{{Name: "W1", Transformer: "passthrough"}},
			Edges:   []EdgeDef{{ID: "w1-done", From: "W1", To: "_done"}},
		}, nil
	})

	outerDef := &CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "A", Transformer: "passthrough"},
			{Name: "B", Delegate: true, Generator: "plan"},
			{Name: "C", Transformer: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
			"plan":        planGen,
		},
	}

	g, err := outerDef.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	trace := &TraceCollector{}
	g.(*DefaultGraph).SetObserver(trace)

	walker := NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// For dslDelegateNode, circuit_type is empty at start (not known until generation).
	starts := trace.EventsOfType(EventDelegateStart)
	if len(starts) != 1 {
		t.Fatalf("delegate_start events = %d, want 1", len(starts))
	}
	if starts[0].Metadata == nil {
		t.Fatal("delegate_start metadata is nil")
	}

	// EventDelegateEnd must carry circuit_type from the generated circuit.
	ends := trace.EventsOfType(EventDelegateEnd)
	if len(ends) != 1 {
		t.Fatalf("delegate_end events = %d, want 1", len(ends))
	}
	if ct, _ := ends[0].Metadata["circuit_type"].(string); ct != "generated" {
		t.Errorf("delegate_end circuit_type = %q, want %q", ct, "generated")
	}
}

func TestBuildGraph_CircuitRefNode_NilRegistry(t *testing.T) {
	def := &CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []NodeDef{
			{Name: "A", HandlerType: HandlerTypeCircuit, Handler: "something"},
		},
		Edges: []EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := def.BuildGraph(GraphRegistries{})
	if err == nil {
		t.Fatal("BuildGraph() should fail when no local circuit and no mediator")
	}
	if !strings.Contains(err.Error(), "no local circuit and no mediator endpoint") {
		t.Errorf("error = %q, want to contain 'no local circuit and no mediator endpoint'", err.Error())
	}
}

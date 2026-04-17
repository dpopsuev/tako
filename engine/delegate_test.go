package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/identity"
)

func TestDelegateArtifact_Interface(t *testing.T) {
	var _ circuit.Artifact = (*DelegateArtifact)(nil)
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
		InnerArtifacts: map[string]circuit.Artifact{
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
		InnerArtifacts: map[string]circuit.Artifact{
			"a": nil,
			"b": nil,
		},
	}
	if got := a.Confidence(); got != 0 {
		t.Errorf("Confidence() = %f, want 0 (all nil)", got)
	}
}

func TestDelegateArtifact_Raw(t *testing.T) {
	inner := map[string]circuit.Artifact{
		"x": &stubArtifact{confidence: 1.0, typ: "t"},
	}
	a := &DelegateArtifact{InnerArtifacts: inner}
	raw, ok := a.Raw().(map[string]circuit.Artifact)
	if !ok {
		t.Fatalf("Raw() type = %T, want map[string]Artifact", a.Raw())
	}
	if len(raw) != 1 {
		t.Errorf("Raw() len = %d, want 1", len(raw))
	}
}

func TestDelegateArtifact_Fields(t *testing.T) {
	def := &circuit.CircuitDef{Circuit: "inner"}
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
	innerDef := &circuit.CircuitDef{
		Circuit: "inner",
		Start:   "X",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "X", Instrument: "transformer", Action: "passthrough"},
			{Name: "Y", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "x-y", From: "X", To: "Y"},
			{ID: "y-done", From: "Y", To: "_done"},
		},
	}

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{name: "Delegate", circuitDef: innerDef}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []circuit.Edge{
		&stubEdge{id: "a-d", from: "A", to: "Delegate"},
		&stubEdge{id: "d-c", from: "Delegate", to: "C"},
		&stubEdge{id: "c-done", from: "C", to: "_done"},
	}

	trace := &TraceCollector{}
	g, err := NewGraph("outer", []circuit.Node{nodeA, nodeD, nodeC}, edges, nil, WithObserver(trace))
	if err != nil {
		t.Fatal(err)
	}

	g.SetRegistries(&GraphRegistries{
		Instruments: InstrumentRegistry{"passthrough": &passthroughTransformer{}},
	})

	walker := &stubWalker{
		identity: identity.Archetype{Name: "test"},
		state:    circuit.NewWalkerState("w1"),
	}

	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
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
	starts := trace.EventsOfType(circuit.EventDelegateStart)
	if len(starts) != 1 {
		t.Errorf("delegate_start events = %d, want 1", len(starts))
	}
	ends := trace.EventsOfType(circuit.EventDelegateEnd)
	if len(ends) != 1 {
		t.Errorf("delegate_end events = %d, want 1", len(ends))
	}

	// Inner walk events should have prefixed node names.
	innerEnters := 0
	for _, ev := range trace.EventsOfType(circuit.EventNodeEnter) {
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

	innerDef := &circuit.CircuitDef{
		Circuit: "inner",
		Start:   "X",
		Done:    "_done",
		Nodes:   []circuit.NodeDef{{Name: "X", Instrument: "transformer", Action: "passthrough"}},
		Edges:   []circuit.EdgeDef{{ID: "x-done", From: "X", To: "_done"}},
	}

	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{name: "D", circuitDef: innerDef}

	edges := []circuit.Edge{
		&stubEdge{id: "a-d", from: "A", to: "D"},
		&stubEdge{id: "d-done", from: "D", to: "_done"},
	}

	g, err := NewGraph("outer", []circuit.Node{nodeA, nodeD}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	walker := &stubWalker{
		identity: identity.Archetype{Name: "test"},
		state:    circuit.NewWalkerState("w1"),
	}

	err = g.Walk(ctx, walker, "A")
	if err == nil {
		t.Fatal("Walk() should fail with canceled context")
	}
}

func TestWalk_DelegateNode_GenerateError(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeD := &testDelegateNode{
		name: "D",
		err:  errors.New("generation failed"),
	}

	edges := []circuit.Edge{
		&stubEdge{id: "a-d", from: "A", to: "D"},
		&stubEdge{id: "d-done", from: "D", to: "_done"},
	}

	g, err := NewGraph("outer", []circuit.Node{nodeA, nodeD}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	walker := &stubWalker{
		identity: identity.Archetype{Name: "test"},
		state:    circuit.NewWalkerState("w1"),
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
	def := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Action: "passthrough", Instrument: "transformer"},
			{Name: "B", Action: "plan", Instrument: "delegate"},
			{Name: "C", Action: "passthrough", Instrument: "transformer"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	planGen := InstrumentFunc("plan", func(_ context.Context, _ *InstrumentContext) (any, error) {
		return &circuit.CircuitDef{
			Circuit: "generated",
			Start:   "W1",
			Done:    "_done",
			Nodes: []circuit.NodeDef{
				{Name: "W1", Instrument: "transformer", Action: "passthrough"},
			},
			Edges: []circuit.EdgeDef{
				{ID: "w1-done", From: "W1", To: "_done"},
			},
		}, nil
	})

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
			"plan":        planGen,
		},
	}

	g, err := BuildGraph(def, reg)
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
	walker := circuit.NewProcessWalker("test")
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

func TestBuildGraph_DelegateNode_MissingHandler(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: "delegate"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := BuildGraph(def, &GraphRegistries{})
	if err == nil {
		t.Fatal("BuildGraph() should fail for delegate without handler")
	}
	if !strings.Contains(err.Error(), "instrument registry is nil") {
		t.Errorf("error = %q, want to contain 'instrument registry is nil'", err.Error())
	}
}

// testDelegateNode is a DelegateNode for compile-time and test-time verification.
type testDelegateNode struct {
	name       string
	circuitDef *circuit.CircuitDef
	err        error
}

func (n *testDelegateNode) Name() string               { return n.name }
func (n *testDelegateNode) Approach() identity.Element { return "" }
func (n *testDelegateNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}
func (n *testDelegateNode) GenerateCircuit(_ context.Context, _ circuit.NodeContext) (*circuit.CircuitDef, error) {
	return n.circuitDef, n.err
}

// --- circuitRefNode tests ---

func TestCircuitRefNode_Interface(t *testing.T) {
	n := &circuitRefNode{baseNode: baseNode{name: "sub"}, circuitDef: &circuit.CircuitDef{Circuit: "inner"}}
	var _ DelegateNode = n
	if n.Name() != "sub" {
		t.Errorf("Name() = %q, want %q", n.Name(), "sub")
	}
}

func TestCircuitRefNode_GenerateCircuit(t *testing.T) {
	inner := &circuit.CircuitDef{Circuit: "beta", Start: "X", Done: "_done"}
	n := &circuitRefNode{baseNode: baseNode{name: "gather"}, circuitDef: inner}

	got, err := n.GenerateCircuit(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("GenerateCircuit() error: %v", err)
	}
	if got != inner {
		t.Error("GenerateCircuit() should return the stored circuit.CircuitDef pointer")
	}
}

func TestBuildGraph_CircuitRefNode(t *testing.T) {
	innerDef := &circuit.CircuitDef{
		Circuit: "beta",
		Start:   "K1",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "K1", Instrument: InstrumentTransformer, Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "alpha",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: "transformer", Action: "passthrough"},
			{Name: "B", Instrument: InstrumentCircuit, Action: "beta"},
			{Name: "C", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*circuit.CircuitDef{
			"beta": innerDef,
		},
	}

	g, err := BuildGraph(outerDef, reg)
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
	innerDef := &circuit.CircuitDef{
		Circuit: "beta",
		Start:   "K1",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "K1", Instrument: InstrumentTransformer, Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "alpha",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: "transformer", Action: "passthrough"},
			{Name: "B", Instrument: InstrumentCircuit, Action: "beta"},
			{Name: "C", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*circuit.CircuitDef{
			"beta": innerDef,
		},
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
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
	if delArt.GeneratedCircuit.Circuit != "beta" {
		t.Errorf("inner circuit = %q, want %q", delArt.GeneratedCircuit.Circuit, "beta")
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
	contextReader := InstrumentFunc("ctx-reader", func(_ context.Context, tc *InstrumentContext) (any, error) {
		v := tc.WalkerState.Context["test-key"]
		return &testArtifact{typeName: "test", confidence: 1.0, raw: v}, nil
	})

	innerDef := &circuit.CircuitDef{
		Circuit: "inner",
		Start:   "R",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "R", Instrument: InstrumentTransformer, Action: "ctx-reader"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "r-done", From: "R", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: "transformer", Action: "passthrough"},
			{Name: "D", Instrument: InstrumentCircuit, Action: "inner"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-d", From: "A", To: "D"},
			{ID: "d-done", From: "D", To: "_done"},
		},
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
			"ctx-reader":  contextReader,
		},
		Circuits: map[string]*circuit.CircuitDef{
			"inner": innerDef,
		},
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
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
	def := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: InstrumentCircuit, Action: "nonexistent"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := BuildGraph(def, &GraphRegistries{
		Circuits: map[string]*circuit.CircuitDef{},
	})
	if err == nil {
		t.Fatal("BuildGraph() should fail for missing circuit reference")
	}
	if !strings.Contains(err.Error(), "no local circuit and no mediator endpoint") {
		t.Errorf("error = %q, want to contain 'no local circuit and no mediator endpoint'", err.Error())
	}
}

func TestDelegateEvents_CarryCircuitType_CircuitRef(t *testing.T) {
	innerDef := &circuit.CircuitDef{
		Circuit: "beta",
		Start:   "K1",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "K1", Instrument: InstrumentTransformer, Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "k1-done", From: "K1", To: "_done"},
		},
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "alpha",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: "transformer", Action: "passthrough"},
			{Name: "B", Instrument: InstrumentCircuit, Action: "beta"},
			{Name: "C", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
		},
		Circuits: map[string]*circuit.CircuitDef{
			"beta": innerDef,
		},
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	trace := &TraceCollector{}
	g.(*DefaultGraph).SetObserver(trace)

	walker := circuit.NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// EventDelegateStart must carry circuit_type.
	starts := trace.EventsOfType(circuit.EventDelegateStart)
	if len(starts) != 1 {
		t.Fatalf("delegate_start events = %d, want 1", len(starts))
	}
	if ct, _ := starts[0].Metadata["circuit_type"].(string); ct != "beta" {
		t.Errorf("delegate_start circuit_type = %q, want %q", ct, "beta")
	}

	// EventDelegateEnd must carry circuit_type.
	ends := trace.EventsOfType(circuit.EventDelegateEnd)
	if len(ends) != 1 {
		t.Fatalf("delegate_end events = %d, want 1", len(ends))
	}
	if ct, _ := ends[0].Metadata["circuit_type"].(string); ct != "beta" {
		t.Errorf("delegate_end circuit_type = %q, want %q", ct, "beta")
	}
}

func TestDelegateEvents_CarryCircuitType_DSLDelegate(t *testing.T) {
	planGen := InstrumentFunc("plan", func(_ context.Context, _ *InstrumentContext) (any, error) {
		return &circuit.CircuitDef{
			Circuit: "generated",
			Start:   "W1",
			Done:    "_done",
			Nodes:   []circuit.NodeDef{{Name: "W1", Instrument: "transformer", Action: "passthrough"}},
			Edges:   []circuit.EdgeDef{{ID: "w1-done", From: "W1", To: "_done"}},
		}, nil
	})

	outerDef := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Action: "passthrough", Instrument: "transformer"},
			{Name: "B", Action: "plan", Instrument: "delegate"},
			{Name: "C", Action: "passthrough", Instrument: "transformer"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "A", To: "B"},
			{ID: "b-c", From: "B", To: "C"},
			{ID: "c-done", From: "C", To: "_done"},
		},
	}

	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{
			"passthrough": &passthroughTransformer{},
			"plan":        planGen,
		},
	}

	g, err := BuildGraph(outerDef, reg)
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}

	trace := &TraceCollector{}
	g.(*DefaultGraph).SetObserver(trace)

	walker := circuit.NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "A"); err != nil {
		t.Fatalf("Walk() error: %v", err)
	}

	// For dslDelegateNode, circuit_type is empty at start (not known until generation).
	starts := trace.EventsOfType(circuit.EventDelegateStart)
	if len(starts) != 1 {
		t.Fatalf("delegate_start events = %d, want 1", len(starts))
	}
	if starts[0].Metadata == nil {
		t.Fatal("delegate_start metadata is nil")
	}

	// EventDelegateEnd must carry circuit_type from the generated circuit.
	ends := trace.EventsOfType(circuit.EventDelegateEnd)
	if len(ends) != 1 {
		t.Fatalf("delegate_end events = %d, want 1", len(ends))
	}
	if ct, _ := ends[0].Metadata["circuit_type"].(string); ct != "generated" {
		t.Errorf("delegate_end circuit_type = %q, want %q", ct, "generated")
	}
}

func TestBuildGraph_CircuitRefNode_NilRegistry(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "outer",
		Start:   "A",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "A", Instrument: InstrumentCircuit, Action: "something"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}

	_, err := BuildGraph(def, &GraphRegistries{})
	if err == nil {
		t.Fatal("BuildGraph() should fail when no local circuit and no mediator")
	}
	if !strings.Contains(err.Error(), "no local circuit and no mediator endpoint") {
		t.Errorf("error = %q, want to contain 'no local circuit and no mediator endpoint'", err.Error())
	}
}

package engine

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func TestTransformerNode_Process(t *testing.T) {
	trans := &echoTransformer{}
	node := &transformerNode{
		name:    "test-node",
		element: circuit.ElementFire,
		trans:   trans,
		config:  map[string]any{"key": "val"},
	}

	if node.Name() != "test-node" {
		t.Errorf("Name() = %q", node.Name())
	}
	if node.ElementAffinity() != circuit.ElementFire {
		t.Errorf("Element = %q", node.ElementAffinity())
	}

	nc := circuit.NodeContext{
		PriorArtifact: &stubArtifact{raw: map[string]any{"data": true}},
	}
	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	m, ok := artifact.Raw().(map[string]any)
	if !ok {
		t.Fatalf("Raw() type = %T", artifact.Raw())
	}
	if m["node"] != "test-node" {
		t.Errorf("node = %v", m["node"])
	}
}

func TestTransformerNode_NilInput(t *testing.T) {
	trans := &echoTransformer{}
	node := &transformerNode{name: "test", trans: trans}

	artifact, err := node.Process(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	m := artifact.Raw().(map[string]any)
	if m["echoed"] != nil {
		t.Errorf("expected nil echoed, got %v", m["echoed"])
	}
}

func TestBuildGraphWith_TransformerNode(t *testing.T) {
	trans := &echoTransformer{}
	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Handler: "echo"},
			{Name: "b", Approach: "analytical", Handler: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "a-to-b", From: "a", To: "b", When: "true"},
			{ID: "E2", Name: "b-to-done", From: "b", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"echo": trans},
	}

	graph, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraphWith: %v", err)
	}

	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if !IsTransformerNode(n) {
			t.Errorf("node %q should be a transformer node", n.Name())
		}
	}
}

func TestBuildGraphWith_MixedTransformerAndWalker(t *testing.T) {
	trans := &echoTransformer{}
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Handler: "echo", HandlerType: "transformer"},
			{Name: "b", Approach: "analytical", Handler: "legacy", HandlerType: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "a-to-b", From: "a", To: "b", When: "true"},
			{ID: "E2", Name: "b-done", From: "b", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	nodeFactory := func(nd circuit.NodeDef) circuit.Node {
		return &testNode{name: nd.Name}
	}

	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"echo": trans},
		Nodes:        NodeRegistry{"legacy": nodeFactory},
	}

	graph, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraphWith: %v", err)
	}

	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if !IsTransformerNode(nodes[0]) {
		t.Error("node a should be transformer")
	}
	if IsTransformerNode(nodes[1]) {
		t.Error("node b should NOT be transformer")
	}
}

type testNode struct {
	name string
}

func (n *testNode) Name() string                     { return n.name }
func (n *testNode) ElementAffinity() circuit.Element { return circuit.ElementFire }
func (n *testNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	return &stubArtifact{raw: map[string]any{"processed": true}}, nil
}

func TestTransformerNode_ResolveInput(t *testing.T) {
	trans := &echoTransformer{}
	node := &transformerNode{
		name:    "triage",
		element: circuit.ElementFire,
		trans:   trans,
		input:   "${recall.output}",
		config:  map[string]any{"key": "val"},
	}

	recallArtifact := &stubArtifact{
		raw: map[string]any{"match": true, "data": "recall-data"},
	}

	state := circuit.NewWalkerState("test")
	state.Outputs["recall"] = recallArtifact

	nc := circuit.NodeContext{
		WalkerState:   state,
		PriorArtifact: &stubArtifact{raw: map[string]any{"prior": "should-be-ignored"}},
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	m := artifact.Raw().(map[string]any)
	echoed, ok := m["echoed"].(map[string]any)
	if !ok {
		t.Fatalf("echoed type = %T, want map[string]any", m["echoed"])
	}
	if echoed["match"] != true {
		t.Errorf("expected recall data, got %v", echoed)
	}
}

func TestTransformerNode_RenderPrompt(t *testing.T) {
	captureNode := &transformerNode{
		name:    "triage",
		element: circuit.ElementFire,
		trans: TransformerFunc("capture", func(_ context.Context, tc *TransformerContext) (any, error) {
			return map[string]any{"prompt": tc.Prompt}, nil
		}),
		prompt: "Analyze {{.Node}} with threshold {{.Config.threshold}}",
		config: map[string]any{"threshold": 0.85},
	}

	state := circuit.NewWalkerState("test")
	nc := circuit.NodeContext{WalkerState: state}

	artifact, err := captureNode.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	m := artifact.Raw().(map[string]any)
	prompt := m["prompt"].(string)
	expected := "Analyze triage with threshold 0.85"
	if prompt != expected {
		t.Errorf("rendered prompt = %q, want %q", prompt, expected)
	}
}

func TestTransformerNode_EmptyInput_FallsBackToPrior(t *testing.T) {
	trans := &echoTransformer{}
	node := &transformerNode{
		name:  "test",
		trans: trans,
	}

	state := circuit.NewWalkerState("test")
	nc := circuit.NodeContext{
		WalkerState:   state,
		PriorArtifact: &stubArtifact{raw: map[string]any{"prior": true}},
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	m := artifact.Raw().(map[string]any)
	echoed, ok := m["echoed"].(map[string]any)
	if !ok {
		t.Fatalf("echoed type = %T, want map[string]any", m["echoed"])
	}
	if echoed["prior"] != true {
		t.Errorf("expected prior artifact data, got %v", echoed)
	}
}

func TestTransformerNode_NodeConfigReachesTransformer(t *testing.T) {
	captureConfig := TransformerFunc("capture-config", func(_ context.Context, tc *TransformerContext) (any, error) {
		return map[string]any{
			"output_path": tc.NodeConfig.OutputPath,
			"max_retries": tc.NodeConfig.MaxRetries,
		}, nil
	})
	node := &transformerNode{
		name:       "test-node",
		element:    circuit.ElementFire,
		trans:      captureConfig,
		nodeConfig: &circuit.NodeConfig{OutputPath: "recall.json", MaxRetries: 3},
	}

	nc := circuit.NodeContext{
		WalkerState: circuit.NewWalkerState("test"),
	}

	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	m := artifact.Raw().(map[string]any)
	if m["output_path"] != "recall.json" {
		t.Errorf("output_path = %v, want recall.json", m["output_path"])
	}
	if m["max_retries"] != 3 {
		t.Errorf("max_retries = %v, want 3", m["max_retries"])
	}
}

func TestNodeDef_ConfigParsedFromYAML(t *testing.T) {
	yamlData := `
circuit: test-config
nodes:
  - name: recall
    element: earth
    transformer: echo
    meta:
      max_retries: 3
      output_path: "output/recall.json"
  - name: triage
    element: fire
    transformer: echo
edges:
  - id: E1
    name: to-triage
    from: recall
    to: triage
    when: "true"
  - id: E2
    name: to-done
    from: triage
    to: _done
    when: "true"
start: recall
done: _done
`
	def, err := circuit.LoadCircuit([]byte(yamlData))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	recallDef := def.Nodes[0]
	cfg := recallDef.EffectiveConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("max_retries = %v, want 3", cfg.MaxRetries)
	}
	if cfg.OutputPath != "output/recall.json" {
		t.Errorf("output_path = %v, want output/recall.json", cfg.OutputPath)
	}

	triageDef := def.Nodes[1]
	if triageDef.Config != nil {
		t.Errorf("triage node Config should be nil, got %v", triageDef.Config)
	}
}

func TestBuildGraph_NodeConfigReachesTransformerContext(t *testing.T) {
	var capturedConfig *circuit.NodeConfig
	captureTrans := TransformerFunc("capture", func(_ context.Context, tc *TransformerContext) (any, error) {
		capturedConfig = tc.NodeConfig
		return map[string]any{"ok": true}, nil
	})

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{
				Name:     "a",
				Approach: "rapid",
				Handler:  "capture",
				Config:   &circuit.NodeConfig{OutputPath: "out.json", MaxRetries: 5},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"capture": captureTrans},
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("NodeConfig was not captured")
	}
	if capturedConfig.OutputPath != "out.json" {
		t.Errorf("OutputPath = %v, want out.json", capturedConfig.OutputPath)
	}
	if capturedConfig.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", capturedConfig.MaxRetries)
	}
}

func TestBuiltinGoTemplate_RendersPrompt(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{
				Name:     "render",
				Approach: "rapid",
				Handler:  "go-template",
				Prompt:   "Hello from {{.Node}}",
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "render", To: "_done", When: "true"},
		},
		Start: "render",
		Done:  "_done",
	}

	capture := NewOutputCapture()
	runner, err := NewRunnerWith(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}
	runner.Graph.(*DefaultGraph).SetObserver(capture)

	err = runner.Walk(context.Background(), nil, "render")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	art, found := capture.ArtifactAt("render")
	if !found {
		t.Fatal("no artifact for render node")
	}
	got, ok := art.Raw().(string)
	if !ok {
		t.Fatalf("artifact Raw() type = %T, want string", art.Raw())
	}
	if got != "Hello from render" {
		t.Errorf("rendered = %q, want %q", got, "Hello from render")
	}
}

func TestBuiltinPassthrough_ReturnsInput(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "source", Approach: "methodical", Handler: "go-template", Prompt: "data"},
			{Name: "pass", Approach: "rapid", Handler: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "to-pass", From: "source", To: "pass", When: "true"},
			{ID: "E2", Name: "done", From: "pass", To: "_done", When: "true"},
		},
		Start: "source",
		Done:  "_done",
	}

	capture := NewOutputCapture()
	runner, err := NewRunnerWith(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}
	runner.Graph.(*DefaultGraph).SetObserver(capture)

	err = runner.Walk(context.Background(), nil, "source")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	art, found := capture.ArtifactAt("pass")
	if !found {
		t.Fatal("no artifact for pass node")
	}
	got, ok := art.Raw().(string)
	if !ok {
		t.Fatalf("artifact Raw() type = %T, want string", art.Raw())
	}
	if got != "data" {
		t.Errorf("passthrough output = %q, want %q", got, "data")
	}
}

func TestBuiltinGoTemplate_NoRegistry(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "a", Approach: "rapid", Handler: "go-template"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	_, err := BuildGraph(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("BuildGraph should succeed for built-in transformer without registry: %v", err)
	}
}

func TestBuiltinTransformer_WithNodeConfig(t *testing.T) {
	var capturedConfig *circuit.NodeConfig
	configCapture := TransformerFunc("config-capture", func(_ context.Context, tc *TransformerContext) (any, error) {
		capturedConfig = tc.NodeConfig
		return tc.Prompt, nil
	})

	def := &circuit.CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{
				Name:     "a",
				Approach: "rapid",
				Handler:  "config-capture",
				Config:   &circuit.NodeConfig{MaxTokens: 1000, ArtifactPath: "/prompts"},
			},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "a", To: "_done", When: "true"},
		},
		Start: "a",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"config-capture": configCapture},
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	err = runner.Walk(context.Background(), nil, "a")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if capturedConfig.ArtifactPath != "/prompts" {
		t.Errorf("ArtifactPath = %v", capturedConfig.ArtifactPath)
	}
	if capturedConfig.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %v", capturedConfig.MaxTokens)
	}
}

func TestTransformerNode_WalkerStateReachesTransformer(t *testing.T) {
	var captured *circuit.WalkerState
	captureTrans := TransformerFunc("capture-state", func(_ context.Context, tc *TransformerContext) (any, error) {
		captured = tc.WalkerState
		return "ok", nil
	})

	node := &transformerNode{
		name:  "test-node",
		trans: captureTrans,
	}

	state := circuit.NewWalkerState("walker-1")
	state.Context["injected"] = "hello"

	nc := circuit.NodeContext{WalkerState: state}
	_, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if captured == nil {
		t.Fatal("WalkerState was not passed to TransformerContext")
	}
	if captured.ID != "walker-1" {
		t.Errorf("WalkerState.ID = %q, want walker-1", captured.ID)
	}
	if captured.Context["injected"] != "hello" {
		t.Errorf("Context[injected] = %v, want hello", captured.Context["injected"])
	}
}

func TestTransformerNode_SlowTransform_ContextDeadline(t *testing.T) {
	slowTrans := TransformerFunc("slow", func(ctx context.Context, tc *TransformerContext) (any, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	def := &circuit.CircuitDef{
		Circuit:     "timeout-test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "slow", Approach: "rapid", Handler: "slow"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "slow", To: "_done", When: "true"},
		},
		Start: "slow",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"slow": slowTrans},
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = runner.Walk(ctx, nil, "slow")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("walk took %v, expected ~100ms abort", elapsed)
	}
}

func TestTransformerNode_ContextCancellation_PropagatesError(t *testing.T) {
	blockingTrans := TransformerFunc("blocking", func(ctx context.Context, tc *TransformerContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	def := &circuit.CircuitDef{
		Circuit:     "cancel-test",
		HandlerType: "transformer",
		Nodes: []circuit.NodeDef{
			{Name: "block", Approach: "rapid", Handler: "blocking"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "block", To: "_done", When: "true"},
		},
		Start: "block",
		Done:  "_done",
	}

	runner, err := NewRunnerWith(def, &GraphRegistries{
		Transformers: TransformerRegistry{"blocking": blockingTrans},
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = runner.Walk(ctx, nil, "block")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("walk took %v, expected ~50ms abort", elapsed)
	}
}

func TestIsTransformerNode(t *testing.T) {
	// Build a real transformer node via BuildGraph.
	def := &circuit.CircuitDef{
		Circuit: "test", Start: "t", Done: "_done",
		HandlerType: "transformer",
		Nodes:       []circuit.NodeDef{{Name: "t", Handler: "echo"}},
		Edges:       []circuit.EdgeDef{{ID: "e", From: "t", To: "_done"}},
	}
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"echo": &echoTransformer{}},
	}
	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatal(err)
	}
	trans, _ := g.NodeByName("t")

	plain := &testNode{name: "p"}

	if !IsTransformerNode(trans) {
		t.Error("expected true for transformerNode")
	}
	if IsTransformerNode(plain) {
		t.Error("expected false for testNode")
	}
}

// --- TypedTransformer tests ---

// typedEchoTransformer expects a map[string]any input.
type typedEchoTransformer struct {
	inputType reflect.Type
}

func (t *typedEchoTransformer) Name() string           { return "typed-echo" }
func (t *typedEchoTransformer) InputType() reflect.Type { return t.inputType }
func (t *typedEchoTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return map[string]any{"echoed": tc.Input, "node": tc.NodeName}, nil
}

func TestTypedTransformer_MatchingInput(t *testing.T) {
	trans := &typedEchoTransformer{inputType: reflect.TypeOf(map[string]any{})}
	node := &transformerNode{
		name:  "typed-node",
		trans: trans,
	}

	nc := circuit.NodeContext{
		PriorArtifact: &stubArtifact{raw: map[string]any{"key": "value"}},
	}
	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process should succeed with matching type: %v", err)
	}
	m := artifact.Raw().(map[string]any)
	if m["node"] != "typed-node" {
		t.Errorf("node = %v, want typed-node", m["node"])
	}
}

func TestTypedTransformer_NilInput(t *testing.T) {
	trans := &typedEchoTransformer{inputType: reflect.TypeOf(map[string]any{})}
	node := &transformerNode{
		name:  "typed-node",
		trans: trans,
	}

	_, err := node.Process(context.Background(), circuit.NodeContext{})
	if err == nil {
		t.Fatal("Process should fail with nil input for TypedTransformer")
	}
	if !strings.Contains(err.Error(), "expected input type") {
		t.Errorf("error should mention expected type, got: %v", err)
	}
	if !strings.Contains(err.Error(), "got nil") {
		t.Errorf("error should mention nil, got: %v", err)
	}
}

func TestTypedTransformer_WrongInputType(t *testing.T) {
	trans := &typedEchoTransformer{inputType: reflect.TypeOf(map[string]any{})}
	node := &transformerNode{
		name:  "typed-node",
		trans: trans,
	}

	nc := circuit.NodeContext{
		PriorArtifact: &stubArtifact{raw: "wrong-type-string"},
	}
	_, err := node.Process(context.Background(), nc)
	if err == nil {
		t.Fatal("Process should fail with wrong input type for TypedTransformer")
	}
	if !strings.Contains(err.Error(), "not assignable to expected") {
		t.Errorf("error should mention assignability, got: %v", err)
	}
}

func TestTypedTransformer_RegularTransformer_NoValidation(t *testing.T) {
	// echoTransformer does NOT implement TypedTransformer — no validation should occur.
	trans := &echoTransformer{}
	node := &transformerNode{
		name:  "untyped-node",
		trans: trans,
	}

	// nil input should pass through without error (backward compatible).
	artifact, err := node.Process(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("Process should succeed for regular Transformer with nil input: %v", err)
	}
	m := artifact.Raw().(map[string]any)
	if m["echoed"] != nil {
		t.Errorf("expected nil echoed, got %v", m["echoed"])
	}
}

func TestTypedTransformer_NilInputType_AcceptsAny(t *testing.T) {
	// TypedTransformer that returns nil InputType — accepts any input.
	trans := &typedEchoTransformer{inputType: nil}
	node := &transformerNode{
		name:  "any-node",
		trans: trans,
	}

	nc := circuit.NodeContext{
		PriorArtifact: &stubArtifact{raw: "anything"},
	}
	artifact, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process should succeed when InputType() returns nil: %v", err)
	}
	m := artifact.Raw().(map[string]any)
	if m["echoed"] != "anything" {
		t.Errorf("echoed = %v, want anything", m["echoed"])
	}
}

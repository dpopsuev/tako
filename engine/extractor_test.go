package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

// stubExtractor is a minimal Extractor for testing.
type stubExtractor struct {
	name string
	fn   func(ctx context.Context, input any) (any, error)
}

func (s *stubExtractor) Name() string {
	return s.name
}

func (s *stubExtractor) Extract(ctx context.Context, input any) (any, error) {
	return s.fn(ctx, input)
}

func TestExtractorRegistry_RegisterAndGet(t *testing.T) {
	reg := make(ExtractorRegistry)
	ext := &stubExtractor{name: "test-ext", fn: func(_ context.Context, in any) (any, error) {
		return in, nil
	}}

	reg.Register(ext)

	got, err := reg.Get("test-ext")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "test-ext" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test-ext")
	}
}

func TestExtractorRegistry_GetUnknown(t *testing.T) {
	reg := make(ExtractorRegistry)
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown extractor")
	}
}

func TestExtractorRegistry_DuplicatePanics(t *testing.T) {
	reg := make(ExtractorRegistry)
	ext := &stubExtractor{name: "dup"}
	reg.Register(ext)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	reg.Register(ext)
}

func TestExtractorNode_Process(t *testing.T) {
	called := false
	ext := &stubExtractor{
		name: "echo",
		fn: func(_ context.Context, in any) (any, error) {
			called = true
			return fmt.Sprintf("echoed: %v", in), nil
		},
	}

	node := &extractorNode{name: "parse", element: "earth", ext: ext}

	if node.Name() != "parse" {
		t.Errorf("Name() = %q, want %q", node.Name(), "parse")
	}
	if node.ElementAffinity() != "earth" {
		t.Errorf("ElementAffinity() = %q, want %q", node.ElementAffinity(), "earth")
	}

	prior := &extractorArtifact{typeName: "raw", raw: "hello"}
	nc := circuit.NodeContext{PriorArtifact: prior}

	art, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !called {
		t.Fatal("extractor was not called")
	}
	if art.Type() != "echo" {
		t.Errorf("Type() = %q, want %q", art.Type(), "echo")
	}
	if art.Raw() != "echoed: hello" {
		t.Errorf("Raw() = %v, want %q", art.Raw(), "echoed: hello")
	}
}

func TestExtractorNode_NilPriorArtifact(t *testing.T) {
	ext := &stubExtractor{
		name: "null-safe",
		fn: func(_ context.Context, in any) (any, error) {
			if in != nil {
				t.Errorf("expected nil input, got %v", in)
			}
			return "ok", nil
		},
	}
	node := &extractorNode{name: "n", ext: ext}
	nc := circuit.NodeContext{PriorArtifact: nil}
	_, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
}

func TestExtractorNode_ExtractError(t *testing.T) {
	ext := &stubExtractor{
		name: "fail",
		fn: func(_ context.Context, _ any) (any, error) {
			return nil, fmt.Errorf("parse failed")
		},
	}
	node := &extractorNode{name: "n", ext: ext}
	_, err := node.Process(context.Background(), circuit.NodeContext{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractorArtifact(t *testing.T) {
	a := &extractorArtifact{typeName: "json", confidence: 0.95, raw: map[string]string{"k": "v"}}
	if a.Type() != "json" {
		t.Errorf("Type() = %q, want %q", a.Type(), "json")
	}
	if a.Confidence() != 0.95 {
		t.Errorf("Confidence() = %f, want 0.95", a.Confidence())
	}
	m, ok := a.Raw().(map[string]string)
	if !ok {
		t.Fatalf("Raw() type = %T, want map[string]string", a.Raw())
	}
	if m["k"] != "v" {
		t.Errorf("Raw()[k] = %q, want %q", m["k"], "v")
	}
}

func TestBuildGraph_WithExtractorNode(t *testing.T) {
	ext := &stubExtractor{
		name: "my-ext",
		fn: func(_ context.Context, in any) (any, error) {
			return "extracted", nil
		},
	}
	extReg := make(ExtractorRegistry)
	extReg.Register(ext)

	nodeReg := NodeRegistry{
		"finish": func(d circuit.NodeDef) circuit.Node {
			return &extTestNode{name: string(d.Name)}
		},
	}

	data := []byte(`
circuit: ext-test
nodes:
  - name: parse
    element: earth
    handler: my-ext
    handler_type: extractor
  - name: done_node
    handler: finish
    handler_type: node
edges:
  - id: E1
    name: parse-to-done
    from: parse
    to: done_node
  - id: E2
    name: to-end
    from: done_node
    to: _done
start: parse
done: _done
`)
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	g, err := BuildGraph(def, &GraphRegistries{Nodes: nodeReg, Extractors: extReg})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	n, ok := g.NodeByName("parse")
	if !ok {
		t.Fatal("node 'parse' not found")
	}
	if n.Name() != "parse" {
		t.Errorf("node name = %q, want %q", n.Name(), "parse")
	}
}

func TestBuildGraph_ExtractorNotRegistered(t *testing.T) {
	extReg := make(ExtractorRegistry)
	nodeReg := NodeRegistry{}

	data := []byte(`
circuit: fail-test
nodes:
  - name: parse
    handler: missing
    handler_type: extractor
edges:
  - id: E1
    name: parse-done
    from: parse
    to: _done
start: parse
done: _done
`)
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	_, err = BuildGraph(def, &GraphRegistries{Nodes: nodeReg, Extractors: extReg})
	if err == nil {
		t.Fatal("expected error for unregistered extractor")
	}
}

func TestLoadCircuit_ExtractorHandler_RoundTrip(t *testing.T) {
	original := &circuit.CircuitDef{
		Circuit:     "ext-roundtrip",
		HandlerType: "extractor",
		Nodes: []circuit.NodeDef{
			{Name: "parse", Approach: "methodical", Handler: "json-v1", HandlerType: "extractor"},
			{Name: "process", Handler: "compute", HandlerType: "node"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "parse-process", From: "parse", To: "process"},
			{ID: "E2", Name: "process-done", From: "process", To: "_done"},
		},
		Start: "parse",
		Done:  "_done",
	}

	data, err := original.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	restored, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	if restored.Nodes[0].Handler != "json-v1" {
		t.Errorf("Nodes[0].Handler = %q, want %q", restored.Nodes[0].Handler, "json-v1")
	}
	if restored.Nodes[1].HandlerType != "node" {
		t.Errorf("Nodes[1].HandlerType = %q, want %q", restored.Nodes[1].HandlerType, "node")
	}
}

func TestJSONSchemaExtractor_ValidInput(t *testing.T) {
	schema := &circuit.ArtifactSchema{
		Type:     "object",
		Required: []string{"test_name", "status"},
		Fields: map[string]circuit.FieldSchema{
			"test_name": {Type: "string"},
			"status":    {Type: "string"},
		},
	}
	ext := &JSONSchemaExtractor{Schema: schema}

	if ext.Name() != "json-schema" {
		t.Errorf("Name() = %q", ext.Name())
	}

	input := `{"test_name":"TestFoo","status":"passed","extra":42}`
	result, err := ext.Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if m["test_name"] != "TestFoo" {
		t.Errorf("test_name = %v", m["test_name"])
	}
}

func TestJSONSchemaExtractor_MissingRequired(t *testing.T) {
	schema := &circuit.ArtifactSchema{
		Type:     "object",
		Required: []string{"test_name"},
		Fields:   map[string]circuit.FieldSchema{"test_name": {Type: "string"}},
	}
	ext := &JSONSchemaExtractor{Schema: schema}

	_, err := ext.Extract(context.Background(), `{"other":"val"}`)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
}

func TestJSONSchemaExtractor_InvalidJSON(t *testing.T) {
	ext := &JSONSchemaExtractor{}

	_, err := ext.Extract(context.Background(), `{invalid}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONSchemaExtractor_ByteSliceInput(t *testing.T) {
	ext := &JSONSchemaExtractor{}

	result, err := ext.Extract(context.Background(), []byte(`{"key":"val"}`))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m := result.(map[string]any)
	if m["key"] != "val" {
		t.Errorf("key = %v", m["key"])
	}
}

func TestJSONSchemaExtractor_NoSchema(t *testing.T) {
	ext := &JSONSchemaExtractor{}

	result, err := ext.Extract(context.Background(), `{"any":"thing"}`)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m := result.(map[string]any)
	if m["any"] != "thing" {
		t.Errorf("any = %v", m["any"])
	}
}

func TestBuildGraph_BuiltinJSONSchemaExtractor(t *testing.T) {
	data := []byte(`
circuit: json-schema-test
nodes:
  - name: parse
    element: earth
    handler: json-schema
    handler_type: extractor
    schema:
      type: object
      required: [name]
      fields:
        name: {type: string}
edges:
  - id: E1
    name: to-done
    from: parse
    to: _done
start: parse
done: _done
`)
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	g, err := BuildGraph(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	n, ok := g.NodeByName("parse")
	if !ok {
		t.Fatal("parse node not found")
	}
	if n.Name() != "parse" {
		t.Errorf("node name = %q, want %q", n.Name(), "parse")
	}

	nc := circuit.NodeContext{
		PriorArtifact: &extractorArtifact{raw: `{"name":"TestFoo"}`},
	}
	art, err := n.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	m := art.Raw().(map[string]any)
	if m["name"] != "TestFoo" {
		t.Errorf("name = %v", m["name"])
	}
}

func TestBuildGraph_BuiltinJSONSchemaExtractor_NoRegistry(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "parse", Approach: "methodical", Handler: "json-schema", HandlerType: "extractor"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "done", From: "parse", To: "_done"},
		},
		Start: "parse",
		Done:  "_done",
	}

	_, err := BuildGraph(def, &GraphRegistries{})
	if err != nil {
		t.Fatalf("BuildGraph should succeed without extractor registry for built-in: %v", err)
	}
}

// extTestNode is a minimal Node for extractor DSL tests.
type extTestNode struct {
	name string
}

func (n *extTestNode) Name() string                    { return n.name }
func (n *extTestNode) ElementAffinity() roster.Element { return "" }
func (n *extTestNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

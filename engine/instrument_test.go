package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
)

// stubDispatcher is a test InstrumentDispatcher with canned responses.
type stubDispatcher struct {
	result json.RawMessage
	err    error
	calls  []json.RawMessage
}

func (d *stubDispatcher) Dispatch(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	d.calls = append(d.calls, input)
	return d.result, d.err
}

func testManifest() *circuit.InstrumentManifest {
	return &circuit.InstrumentManifest{
		Kind:      circuit.KindInstrument,
		Name:      "test-instrument",
		Namespace: "test",
		Dispatch:  circuit.DispatchCLI,
		Tune:      "true",
		Actions: map[string]def.ActionDef{
			"echo": {Command: "echo test"},
		},
	}
}

func TestInstrumentNode_Name(t *testing.T) {
	n := &instrumentNode{name: "scan"}
	if n.Name() != "scan" {
		t.Errorf("Name() = %q, want %q", n.Name(), "scan")
	}
}

func TestInstrumentNode_Process_ReturnsArtifact(t *testing.T) {
	disp := &stubDispatcher{result: json.RawMessage(`{"ok":true}`)}
	n := &instrumentNode{
		name:       "scan",
		manifest:   testManifest(),
		actionName: "echo",
		dispatcher: disp,
	}

	art, err := n.Process(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if art == nil {
		t.Fatal("expected non-nil artifact")
	}
	if art.Type() != "instrument:test-instrument:echo" {
		t.Errorf("Type() = %q", art.Type())
	}
	if art.Confidence() != 1.0 {
		t.Errorf("Confidence() = %f", art.Confidence())
	}
	raw, ok := art.Raw().(map[string]any)
	if !ok {
		t.Fatalf("Raw() type = %T, want map[string]any", art.Raw())
	}
	if raw["ok"] != true {
		t.Errorf("Raw() = %v", raw)
	}
}

func TestInstrumentNode_Process_DispatcherError(t *testing.T) {
	disp := &stubDispatcher{err: errors.New("command failed")}
	n := &instrumentNode{
		name:       "scan",
		manifest:   testManifest(),
		actionName: "echo",
		dispatcher: disp,
	}

	_, err := n.Process(context.Background(), circuit.NodeContext{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInstrumentDispatch) {
		t.Errorf("want ErrInstrumentDispatch, got %v", err)
	}
}

func TestInstrumentNode_Process_ResolvesInput(t *testing.T) {
	disp := &stubDispatcher{result: json.RawMessage(`{}`)}
	n := &instrumentNode{
		name:       "triage",
		manifest:   testManifest(),
		actionName: "echo",
		dispatcher: disp,
		input:      "${scan.output}",
	}

	ws := circuit.NewWalkerState("test")
	ws.Outputs["scan"] = &testArtifact{typeName: "test", confidence: 1.0, raw: map[string]any{"findings": 3}}

	_, err := n.Process(context.Background(), circuit.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(disp.calls) != 1 {
		t.Fatalf("expected 1 dispatch call, got %d", len(disp.calls))
	}

	var payload map[string]any
	if err := json.Unmarshal(disp.calls[0], &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	inp, ok := payload["input"].(map[string]any)
	if !ok {
		t.Fatalf("payload.input type = %T, want map", payload["input"])
	}
	if inp["findings"] != float64(3) {
		t.Errorf("payload.input.findings = %v", inp["findings"])
	}
}

func TestInstrumentNode_Process_NilWalkerState(t *testing.T) {
	disp := &stubDispatcher{result: json.RawMessage(`{}`)}
	n := &instrumentNode{
		name:       "scan",
		manifest:   testManifest(),
		actionName: "echo",
		dispatcher: disp,
	}

	art, err := n.Process(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("Process with nil walker: %v", err)
	}
	if art == nil {
		t.Fatal("expected non-nil artifact")
	}
}

func TestInstrumentNode_Process_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	disp := &stubDispatcher{err: context.Canceled}
	n := &instrumentNode{
		name:       "scan",
		manifest:   testManifest(),
		actionName: "echo",
		dispatcher: disp,
	}

	_, err := n.Process(ctx, circuit.NodeContext{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestResolveInstrumentNode_ValidManifest(t *testing.T) {
	manifest := testManifest()
	nd := &circuit.NodeDef{Name: "scan", Action: "echo"}
	def := &circuit.CircuitDef{}

	node, err := resolveInstrumentNode(def, nd, manifest, "", "")
	if err != nil {
		t.Fatalf("resolveInstrumentNode: %v", err)
	}
	if node.Name() != "scan" {
		t.Errorf("Name() = %q", node.Name())
	}
}

func TestResolveInstrumentNode_UnknownAction(t *testing.T) {
	manifest := testManifest()
	nd := &circuit.NodeDef{Name: "scan", Action: "nonexistent"}
	def := &circuit.CircuitDef{}

	_, err := resolveInstrumentNode(def, nd, manifest, "", "")
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !errors.Is(err, ErrInstrument) {
		t.Errorf("want ErrInstrument, got %v", err)
	}
}

func TestResolveInstrumentNode_ActionDefaultsToNodeName(t *testing.T) {
	manifest := &circuit.InstrumentManifest{
		Name:     "test",
		Dispatch: circuit.DispatchCLI,
		Tune:     "true",
		Actions:  map[string]def.ActionDef{"scan": {Command: "echo"}},
	}
	nd := &circuit.NodeDef{Name: "scan"} // no explicit Action
	d := &circuit.CircuitDef{}

	node, err := resolveInstrumentNode(d, nd, manifest, "", "")
	if err != nil {
		t.Fatalf("resolveInstrumentNode: %v", err)
	}
	if node.Name() != "scan" {
		t.Errorf("Name() = %q", node.Name())
	}
}

func TestResolveByInstrument_PrefersRegistry(t *testing.T) {
	manifest := testManifest()
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{"test-instrument": manifest},
	}
	nd := &circuit.NodeDef{Name: "scan", Instrument: "test-instrument", Action: "echo"}
	def := &circuit.CircuitDef{}

	node, err := resolveByInstrument(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveByInstrument: %v", err)
	}
	if _, ok := node.(*instrumentNode); !ok {
		t.Errorf("expected *instrumentNode, got %T", node)
	}
}

func TestResolveByInstrument_FallsBackToBuiltin(t *testing.T) {
	stub := TransformerFunc("echo", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "ok", nil
	})
	reg := &GraphRegistries{
		Transformers: TransformerRegistry{"echo": stub},
		Instruments:  InstrumentRegistry{}, // empty — no manifests
	}
	nd := &circuit.NodeDef{Name: "scan", Instrument: "transformer", Action: "echo"}
	def := &circuit.CircuitDef{}

	node, err := resolveByInstrument(def, nd, reg, "")
	if err != nil {
		t.Fatalf("resolveByInstrument: %v", err)
	}
	// Should be a transformerNode, not instrumentNode.
	if _, ok := node.(*instrumentNode); ok {
		t.Error("expected bridge resolution, not instrument resolution")
	}
}

func TestResolveByInstrument_UnsupportedDispatchMode(t *testing.T) {
	manifest := &circuit.InstrumentManifest{
		Name:     "remote",
		Dispatch: circuit.DispatchMCP,
		Tune:     "true",
		Actions:  map[string]def.ActionDef{"call": {Command: ""}},
	}
	reg := &GraphRegistries{
		Instruments: InstrumentRegistry{"remote": manifest},
	}
	nd := &circuit.NodeDef{Name: "rpc", Instrument: "remote", Action: "call"}
	def := &circuit.CircuitDef{}

	_, err := resolveByInstrument(def, nd, reg, "")
	if err == nil {
		t.Fatal("expected error for unsupported dispatch mode")
	}
}

func TestInstrumentArtifact_Interface(t *testing.T) {
	var _ circuit.Artifact = (*instrumentArtifact)(nil)
}

func TestBuildInstrumentPayload_Empty(t *testing.T) {
	p, err := buildInstrumentPayload(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(p) != `{}` {
		t.Errorf("got %s", p)
	}
}

func TestBuildInstrumentPayload_WithInput(t *testing.T) {
	p, err := buildInstrumentPayload(map[string]any{"key": "val"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(p, &m)
	if m["input"] == nil {
		t.Error("expected input in payload")
	}
}

func TestBuildInstrumentPayload_WithPrompt(t *testing.T) {
	p, err := buildInstrumentPayload(nil, "analyze this")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(p, &m)
	if m["prompt"] != "analyze this" {
		t.Errorf("prompt = %v", m["prompt"])
	}
}

func TestParseInstrumentOutput_JSON(t *testing.T) {
	out := parseInstrumentOutput(json.RawMessage(`{"ok":true}`))
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("type = %T, want map", out)
	}
	if m["ok"] != true {
		t.Errorf("ok = %v", m["ok"])
	}
}

func TestParseInstrumentOutput_String(t *testing.T) {
	out := parseInstrumentOutput(json.RawMessage(`not json`))
	s, ok := out.(string)
	if !ok {
		t.Fatalf("type = %T, want string", out)
	}
	if s != "not json" {
		t.Errorf("got %q", s)
	}
}

func TestParseInstrumentOutput_Empty(t *testing.T) {
	out := parseInstrumentOutput(nil)
	if out != nil {
		t.Errorf("expected nil, got %v", out)
	}
}

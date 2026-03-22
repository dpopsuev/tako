package framework

import (
	"testing"
)

type testArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *testArtifact) Type() string       { return a.typeName }
func (a *testArtifact) Confidence() float64 { return a.confidence }
func (a *testArtifact) Raw() any            { return a.raw }

func TestExpressionEdge_CompileValid(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.match == true && output.confidence >= 0.8`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if edge.ID() != "E1" {
		t.Errorf("ID = %q, want E1", edge.ID())
	}
}

func TestExpressionEdge_CompileInvalid(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.field >>>`,
	}
	_, err := CompileExpressionEdge(def)
	if err == nil {
		t.Fatal("expected compile error for invalid expression")
	}
}

func TestExpressionEdge_CompileEmpty(t *testing.T) {
	def := EdgeDef{ID: "E1", From: "a", To: "b", When: ""}
	_, err := CompileExpressionEdge(def)
	if err == nil {
		t.Fatal("expected error for empty When")
	}
}

func TestExpressionEdge_EvaluateMatch(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.match == true && output.confidence >= 0.8`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	artifact := &testArtifact{raw: map[string]any{"match": true, "confidence": 0.85}}
	state := NewWalkerState("test")
	tr := edge.Evaluate(artifact, state)
	if tr == nil {
		t.Fatal("expected transition, got nil")
	}
	if tr.NextNode != "b" {
		t.Errorf("NextNode = %q, want b", tr.NextNode)
	}
}

func TestExpressionEdge_EvaluateNoMatch(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.match == true && output.confidence >= 0.8`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	artifact := &testArtifact{raw: map[string]any{"match": false, "confidence": 0.5}}
	state := NewWalkerState("test")
	tr := edge.Evaluate(artifact, state)
	if tr != nil {
		t.Fatalf("expected nil transition, got %+v", tr)
	}
}

func TestExpressionEdge_LoopCountAccess(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b", Loop: true,
		When: `state.loops.investigate < 3`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	state := NewWalkerState("test")
	state.LoopCounts["investigate"] = 2

	artifact := &testArtifact{raw: map[string]any{}}
	tr := edge.Evaluate(artifact, state)
	if tr == nil {
		t.Fatal("expected transition (loops=2 < 3)")
	}
	if !edge.IsLoop() {
		t.Error("IsLoop should be true")
	}

	state.LoopCounts["investigate"] = 3
	tr = edge.Evaluate(artifact, state)
	if tr != nil {
		t.Fatal("expected nil transition (loops=3 >= 3)")
	}
}

func TestExpressionEdge_ConfigAccess(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.confidence >= config.threshold`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	artifact := &testArtifact{raw: map[string]any{"confidence": 0.9}}
	state := NewWalkerState("test")

	// expressionEdge currently passes nil config; test with direct context
	ctx := buildExprContext(artifact, state, map[string]any{"threshold": 0.8})
	result, err := runExprProgram(edge.Program(), ctx)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != true {
		t.Error("expected true (0.9 >= 0.8)")
	}
}

func TestExpressionEdge_NilArtifact(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `len(output) == 0`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	tr := edge.Evaluate(nil, NewWalkerState("test"))
	if tr == nil {
		t.Fatal("expected transition for nil artifact (empty output)")
	}
}

func TestExpressionEdge_StructArtifact(t *testing.T) {
	type recall struct {
		Match      bool    `json:"match"`
		Confidence float64 `json:"confidence"`
	}
	def := EdgeDef{
		ID: "E1", From: "a", To: "b",
		When: `output.match == true`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	artifact := &testArtifact{raw: &recall{Match: true, Confidence: 0.9}}
	tr := edge.Evaluate(artifact, NewWalkerState("test"))
	if tr == nil {
		t.Fatal("expected transition for struct artifact")
	}
}

func TestExpressionEdge_ShortcutFlag(t *testing.T) {
	def := EdgeDef{
		ID: "E1", From: "a", To: "b", Shortcut: true,
		When: `true`,
	}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !edge.IsShortcut() {
		t.Error("IsShortcut should be true")
	}
}

func TestArtifactToMap_NilRaw(t *testing.T) {
	m := artifactToMap(&testArtifact{raw: nil})
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestArtifactToMap_MapPassthrough(t *testing.T) {
	input := map[string]any{"key": "value"}
	m := artifactToMap(&testArtifact{raw: input})
	if m["key"] != "value" {
		t.Errorf("expected passthrough, got %v", m)
	}
}

func TestBuildExprContext_NilState(t *testing.T) {
	ctx := buildExprContext(nil, nil, nil)
	if ctx.State.Loops == nil {
		t.Error("Loops should be initialized even with nil state")
	}
	if ctx.Config == nil {
		t.Error("Config should be initialized even with nil config")
	}
}

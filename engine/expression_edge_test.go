package engine

import (
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestCompileExpressionEdge_ValidExpression(t *testing.T) {
	def := &circuit.EdgeDef{
		ID:   "e1",
		From: "a",
		To:   "b",
		When: "output.skip != true",
	}

	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("CompileExpressionEdge: %v", err)
	}
	if edge.ID() != "e1" {
		t.Errorf("ID = %q, want e1", edge.ID())
	}
	if edge.From() != "a" {
		t.Errorf("From = %q, want a", edge.From())
	}
	if edge.To() != "b" {
		t.Errorf("To = %q, want b", edge.To())
	}
}

func TestCompileExpressionEdge_EmptyWhen(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", When: ""}
	_, err := CompileExpressionEdge(def)
	if err == nil {
		t.Fatal("expected error for empty When")
	}
	if !errors.Is(err, ErrEdge) {
		t.Errorf("want ErrEdge, got %v", err)
	}
}

func TestCompileExpressionEdge_InvalidExpression(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", When: "not_a_valid_expr(((("}
	_, err := CompileExpressionEdge(def)
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestExpressionEdge_Evaluate_TrueCondition(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", From: "a", To: "b", When: "true"}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	transition := edge.Evaluate(nil, circuit.NewWalkerState("test"))
	if transition == nil {
		t.Fatal("expected non-nil transition for true condition")
	}
	if transition.NextNode != "b" {
		t.Errorf("NextNode = %q, want b", transition.NextNode)
	}
}

func TestExpressionEdge_Evaluate_FalseCondition(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", From: "a", To: "b", When: "false"}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	transition := edge.Evaluate(nil, circuit.NewWalkerState("test"))
	if transition != nil {
		t.Error("expected nil transition for false condition")
	}
}

func TestExpressionEdge_Evaluate_OutputField(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", From: "a", To: "b", When: "output.skip == true"}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Artifact with skip=true
	art := &simpleArtifact{raw: map[string]any{"skip": true}}
	transition := edge.Evaluate(art, circuit.NewWalkerState("test"))
	if transition == nil {
		t.Fatal("expected match when output.skip == true")
	}

	// Artifact with skip=false
	art2 := &simpleArtifact{raw: map[string]any{"skip": false}}
	transition2 := edge.Evaluate(art2, circuit.NewWalkerState("test"))
	if transition2 != nil {
		t.Error("expected no match when output.skip == false")
	}
}

func TestExpressionEdge_Evaluate_LoopCount(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", From: "a", To: "b", When: "state.loops.a < 3"}
	edge, err := CompileExpressionEdge(def)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	state := circuit.NewWalkerState("test")
	state.LoopCounts["a"] = 1

	transition := edge.Evaluate(nil, state)
	if transition == nil {
		t.Fatal("expected match when loops.a < 3")
	}

	state.LoopCounts["a"] = 5
	transition2 := edge.Evaluate(nil, state)
	if transition2 != nil {
		t.Error("expected no match when loops.a >= 3")
	}
}

func TestExpressionEdge_Evaluate_ConfigAccess(t *testing.T) {
	def := &circuit.EdgeDef{ID: "e1", From: "a", To: "b", When: "config.threshold > 0.5"}
	cfg := map[string]any{"threshold": 0.8}
	edge, err := CompileExpressionEdge(def, cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	transition := edge.Evaluate(nil, circuit.NewWalkerState("test"))
	if transition == nil {
		t.Fatal("expected match when config.threshold > 0.5")
	}
}

// simpleArtifact is a test helper that returns raw as map.
type simpleArtifact struct {
	raw any
}

func (a *simpleArtifact) Type() string        { return "test" }
func (a *simpleArtifact) Confidence() float64 { return 1.0 }
func (a *simpleArtifact) Raw() any            { return a.raw }

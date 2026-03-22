package circuit

import (
	"testing"
)

func TestResolveInput_Valid(t *testing.T) {
	outputs := map[string]Artifact{
		"recall": &testArtifact{typeName: "recall", raw: map[string]any{"match": true}},
	}

	art, err := ResolveInput("${recall.output}", outputs)
	if err != nil {
		t.Fatalf("ResolveInput: %v", err)
	}
	if art == nil {
		t.Fatal("expected artifact, got nil")
	}
	if art.Type() != "recall" {
		t.Errorf("Type() = %q, want recall", art.Type())
	}
}

func TestResolveInput_Empty(t *testing.T) {
	art, err := ResolveInput("", nil)
	if err != nil {
		t.Fatalf("ResolveInput: %v", err)
	}
	if art != nil {
		t.Errorf("expected nil for empty input, got %v", art)
	}
}

func TestResolveInput_InvalidFormat(t *testing.T) {
	_, err := ResolveInput("not-a-reference", nil)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestResolveInput_MissingNode(t *testing.T) {
	outputs := map[string]Artifact{}
	_, err := ResolveInput("${triage.output}", outputs)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestResolveInput_Whitespace(t *testing.T) {
	outputs := map[string]Artifact{
		"recall": &testArtifact{typeName: "recall", raw: "data"},
	}
	art, err := ResolveInput("  ${recall.output}  ", outputs)
	if err != nil {
		t.Fatalf("ResolveInput: %v", err)
	}
	if art == nil {
		t.Fatal("expected artifact")
	}
}

func TestRenderPrompt_Basic(t *testing.T) {
	tc := TemplateContext{
		Output: map[string]any{"key": "value"},
		Config: map[string]any{"threshold": 0.8},
		Node:   "triage",
	}
	result, err := RenderPrompt("Node: {{.Node}}, threshold: {{.Config.threshold}}", tc)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	if result != "Node: triage, threshold: 0.8" {
		t.Errorf("result = %q", result)
	}
}

func TestRenderPrompt_MissingKey(t *testing.T) {
	tc := TemplateContext{Config: map[string]any{}}
	result, err := RenderPrompt("val={{.Config.missing}}", tc)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	if result != "val=<no value>" {
		t.Errorf("result = %q", result)
	}
}

func TestRenderPrompt_InvalidTemplate(t *testing.T) {
	_, err := RenderPrompt("{{.Unclosed", TemplateContext{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestMergeVars_Basic(t *testing.T) {
	base := map[string]any{"a": 1, "b": 2}
	overrides := map[string]any{"b": 99, "c": 3}
	result := MergeVars(base, overrides)

	if result["a"] != 1 {
		t.Errorf("a = %v, want 1", result["a"])
	}
	if result["b"] != 99 {
		t.Errorf("b = %v, want 99", result["b"])
	}
	if result["c"] != 3 {
		t.Errorf("c = %v, want 3", result["c"])
	}
}

func TestMergeVars_NilBase(t *testing.T) {
	result := MergeVars(nil, map[string]any{"x": 1})
	if result["x"] != 1 {
		t.Errorf("x = %v, want 1", result["x"])
	}
}

func TestMergeVars_NilOverrides(t *testing.T) {
	base := map[string]any{"a": 1}
	result := MergeVars(base, nil)
	if result["a"] != 1 {
		t.Errorf("a = %v, want 1", result["a"])
	}
}

func TestMergeVars_DoesNotMutateBase(t *testing.T) {
	base := map[string]any{"a": 1}
	MergeVars(base, map[string]any{"a": 2})
	if base["a"] != 1 {
		t.Errorf("base mutated: a = %v, want 1", base["a"])
	}
}

package transformers

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/engine"
)

type stubDispatcher struct{}

//nolint:gocritic // hugeParam: interface conformance (agentport.Dispatcher)
func (stubDispatcher) Dispatch(_ context.Context, _ agentport.Context) ([]byte, error) {
	return []byte(`{}`), nil
}

func TestCoreComponent_RegistersBuiltins(t *testing.T) {
	comp := CoreComponent(nil)

	if comp.Namespace != "core" {
		t.Errorf("Namespace = %q, want core", comp.Namespace)
	}
	if comp.Name != "origami-core" {
		t.Errorf("Name = %q, want origami-core", comp.Name)
	}

	expected := []string{"http", "jq", "file", "template-params", "match"}
	for _, name := range expected {
		if _, ok := comp.Transformers[name]; !ok {
			t.Errorf("missing transformer %q", name)
		}
	}

	// llm should NOT be registered when dispatcher is nil
	if _, ok := comp.Transformers["llm"]; ok {
		t.Error("llm should not be registered when dispatcher is nil")
	}
}

func TestCoreComponent_WithDispatcher(t *testing.T) {
	d := stubDispatcher{}
	comp := CoreComponent(d)

	if _, ok := comp.Transformers["llm"]; !ok {
		t.Error("llm should be registered when dispatcher is provided")
	}
}

func TestWithCoreBaseDir(t *testing.T) {
	comp := CoreComponent(nil, WithCoreBaseDir("/tmp/test"))

	// Verify it doesn't panic and the component is valid
	if comp.Namespace != "core" {
		t.Errorf("Namespace = %q, want core", comp.Namespace)
	}
	if len(comp.Transformers) == 0 {
		t.Error("no transformers registered")
	}
}

func TestTemplateParamsTransformer_Basic(t *testing.T) {
	tp := NewTemplateParams()

	if tp.Name() != "template-params" {
		t.Errorf("Name() = %q, want template-params", tp.Name())
	}
	if !tp.Deterministic() {
		t.Error("Deterministic() should return true")
	}

	tc := &engine.TransformerContext{
		NodeName: "build-context",
		Config:   map[string]any{"env": "prod"},
		Meta: map[string]any{
			"include_config": true,
			"extra":          map[string]any{"step": "recall"},
		},
	}

	result, err := tp.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want map[string]any", result)
	}
	if m["env"] != "prod" {
		t.Errorf("env = %v, want prod", m["env"])
	}
	if m["step"] != "recall" {
		t.Errorf("step = %v, want recall", m["step"])
	}
	if m["node"] != "build-context" {
		t.Errorf("node = %v, want build-context", m["node"])
	}
}

func TestTemplateParamsTransformer_IncludeState(t *testing.T) {
	tp := NewTemplateParams()

	tc := &engine.TransformerContext{
		NodeName: "merge",
		Input:    map[string]any{"findings": []string{"f1"}},
		Meta: map[string]any{
			"include_state": true,
		},
	}

	result, err := tp.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	m := result.(map[string]any)
	if _, ok := m["findings"]; !ok {
		t.Error("input state not merged into params")
	}
}

func TestTemplateParamsTransformer_Pick(t *testing.T) {
	tp := NewTemplateParams()

	tc := &engine.TransformerContext{
		NodeName: "filter",
		Config:   map[string]any{"env": "prod", "debug": true, "region": "us"},
		Meta: map[string]any{
			"include_config": true,
			"pick":           []any{"env", "region"},
		},
	}

	result, err := tp.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	m := result.(map[string]any)
	if m["env"] != "prod" {
		t.Errorf("env = %v, want prod", m["env"])
	}
	if m["region"] != "us" {
		t.Errorf("region = %v, want us", m["region"])
	}
	if _, ok := m["debug"]; ok {
		t.Error("debug should be filtered out by pick")
	}
}

package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

type stubRenderer struct {
	name   string
	output string
}

func (s *stubRenderer) Name() string                                    { return s.name }
func (s *stubRenderer) Render(_ context.Context, _ any) (string, error) { return s.output, nil }

func TestRendererRegistry_Get(t *testing.T) {
	reg := RendererRegistry{}
	reg.Register(&stubRenderer{name: "narrative-v1", output: "hello"})

	rnd, err := reg.Get("narrative-v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rnd.Name() != "narrative-v1" {
		t.Fatalf("got name %q, want narrative-v1", rnd.Name())
	}
}

func TestRendererRegistry_GetFQCN(t *testing.T) {
	reg := RendererRegistry{}
	reg.Register(&stubRenderer{name: "vendor.narrative-v1", output: "hello"})

	rnd, err := reg.Get("narrative-v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rnd.Name() != "vendor.narrative-v1" {
		t.Fatalf("got name %q, want vendor.narrative-v1", rnd.Name())
	}
}

func TestRendererRegistry_GetNotFound(t *testing.T) {
	reg := RendererRegistry{}
	_, err := reg.Get("missing")
	if err == nil {
		t.Fatal("expected error for missing renderer")
	}
}

func TestRendererRegistry_RegisterDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	reg := RendererRegistry{}
	reg.Register(&stubRenderer{name: "dup"})
	reg.Register(&stubRenderer{name: "dup"})
}

func TestTemplateRenderer_Render(t *testing.T) {
	rnd := &TemplateRenderer{Template: "Hello {{.Node}}"}
	result, err := rnd.Render(context.Background(), TemplateContext{Node: "test-node"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello test-node" {
		t.Fatalf("got %q, want %q", result, "Hello test-node")
	}
}

func TestTemplateRenderer_BadInput(t *testing.T) {
	rnd := &TemplateRenderer{Template: "Hello"}
	_, err := rnd.Render(context.Background(), "not-a-template-context")
	if err == nil {
		t.Fatal("expected error for non-TemplateContext input")
	}
}

func TestRendererNode_Process(t *testing.T) {
	rnd := &stubRenderer{name: "test", output: "rendered text"}
	node := &rendererNode{name: "render-node", rnd: rnd}

	art, err := node.Process(context.Background(), circuit.NodeContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if art.Type() != "test" {
		t.Fatalf("got type %q, want test", art.Type())
	}
	if art.Raw() != "rendered text" {
		t.Fatalf("got raw %q, want 'rendered text'", art.Raw())
	}
}

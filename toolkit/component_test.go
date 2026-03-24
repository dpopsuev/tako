package toolkit

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

type stubTransformer struct{ name string }

func (s *stubTransformer) Name() string { return s.name }
func (s *stubTransformer) Transform(_ context.Context, _ *engine.TransformerContext) (any, error) {
	return nil, nil
}

type stubExtractor struct{ name string }

func (s *stubExtractor) Name() string { return s.name }
func (s *stubExtractor) Extract(_ context.Context, _ any) (any, error) {
	return nil, nil
}

func TestTransformerForAllNodes(t *testing.T) {
	t.Parallel()
	tr := &stubTransformer{name: "stub"}
	nodes := []string{"a", "b", "c"}

	reg := TransformerForAllNodes(tr, nodes)
	if len(reg) != 3 {
		t.Fatalf("registry len = %d, want 3", len(reg))
	}
	for _, name := range nodes {
		if reg[name] != tr {
			t.Errorf("reg[%s] not pointing to the stub transformer", name)
		}
	}
}

func TestTransformerForAllNodes_Empty(t *testing.T) {
	t.Parallel()
	reg := TransformerForAllNodes(&stubTransformer{}, nil)
	if len(reg) != 0 {
		t.Errorf("empty nodes should give empty registry, got %d", len(reg))
	}
}

func TestExtractorForAllNodes(t *testing.T) {
	t.Parallel()
	nodes := []string{"x", "y"}
	factory := func(name string) engine.Extractor {
		return &stubExtractor{name: name}
	}

	reg := ExtractorForAllNodes(factory, nodes)
	if len(reg) != 2 {
		t.Fatalf("registry len = %d, want 2", len(reg))
	}
	for _, name := range nodes {
		ext, ok := reg[name]
		if !ok {
			t.Errorf("missing extractor for %q", name)
			continue
		}
		if ext.Name() != name {
			t.Errorf("extractor name = %q, want %q", ext.Name(), name)
		}
	}
}

func TestNodeNamesFromCircuit(t *testing.T) {
	t.Parallel()
	cd := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
			{Name: "resolve"},
		},
	}

	names := NodeNamesFromCircuit(cd)
	if len(names) != 3 {
		t.Fatalf("len = %d, want 3", len(names))
	}
	want := []string{"recall", "triage", "resolve"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

func TestNodeNamesFromCircuit_Nil(t *testing.T) {
	t.Parallel()
	if got := NodeNamesFromCircuit(nil); got != nil {
		t.Errorf("nil circuit should return nil, got %v", got)
	}
}

func TestNodeNamesFromCircuit_Empty(t *testing.T) {
	t.Parallel()
	cd := &circuit.CircuitDef{}
	got := NodeNamesFromCircuit(cd)
	if len(got) != 0 {
		t.Errorf("empty nodes should return empty slice, got %v", got)
	}
}

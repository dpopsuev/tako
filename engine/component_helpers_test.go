package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

type componentStubTransformer struct{ name string }

func (s *componentStubTransformer) Name() string { return s.name }
func (s *componentStubTransformer) Transform(_ context.Context, _ *InstrumentContext) (any, error) {
	return nil, nil
}

type componentStubExtractor struct{ name string }

func (s *componentStubExtractor) Name() string { return s.name }
func (s *componentStubExtractor) Extract(_ context.Context, _ any) (any, error) {
	return nil, nil
}

func TestInstrumentForAllNodes(t *testing.T) {
	t.Parallel()
	tr := &componentStubTransformer{name: "stub"}
	nodes := []string{"a", "b", "c"}

	reg := InstrumentForAllNodes(tr, nodes)
	if len(reg) != 3 {
		t.Fatalf("registry len = %d, want 3", len(reg))
	}
	for _, name := range nodes {
		if reg[name] != tr {
			t.Errorf("reg[%s] not pointing to the stub transformer", name)
		}
	}
}

func TestInstrumentForAllNodes_Empty(t *testing.T) {
	t.Parallel()
	reg := InstrumentForAllNodes(&componentStubTransformer{}, nil)
	if len(reg) != 0 {
		t.Errorf("empty nodes should give empty registry, got %d", len(reg))
	}
}

func TestExtractorForAllNodes(t *testing.T) {
	t.Parallel()
	nodes := []string{"x", "y"}
	factory := func(name string) Extractor {
		return &componentStubExtractor{name: name}
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

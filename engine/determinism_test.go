package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

type deterministicStub struct{ name string }

func (d *deterministicStub) Name() string        { return d.name }
func (d *deterministicStub) Deterministic() bool { return true }
func (d *deterministicStub) Transform(_ context.Context, _ *TransformerContext) (any, error) {
	return nil, nil
}

type stochasticStub struct{ name string }

func (s *stochasticStub) Name() string        { return s.name }
func (s *stochasticStub) Deterministic() bool { return false }
func (s *stochasticStub) Transform(_ context.Context, _ *TransformerContext) (any, error) {
	return nil, nil
}

type unknownStub struct{ name string }

func (u *unknownStub) Name() string { return u.name }
func (u *unknownStub) Transform(_ context.Context, _ *TransformerContext) (any, error) {
	return nil, nil
}

func TestIsDeterministic(t *testing.T) {
	if !IsDeterministic(&deterministicStub{}) {
		t.Error("expected deterministic stub to return true")
	}
	if IsDeterministic(&stochasticStub{}) {
		t.Error("expected stochastic stub to return false")
	}
	if IsDeterministic(&unknownStub{}) {
		t.Error("expected unknown stub to default to false (stochastic)")
	}
}

func TestIsCircuitDeterministic(t *testing.T) {
	reg := TransformerRegistry{
		"core.jq":  &deterministicStub{name: "core.jq"},
		"core.llm": &stochasticStub{name: "core.llm"},
	}

	tests := []struct {
		name  string
		nodes []circuit.NodeDef
		want  bool
	}{
		{
			name: "all deterministic",
			nodes: []circuit.NodeDef{
				{Name: "a", Action: "core.jq", Instrument: "transformer"},
				{Name: "b", Action: "core.jq", Instrument: "transformer"},
			},
			want: true,
		},
		{
			name: "one stochastic",
			nodes: []circuit.NodeDef{
				{Name: "a", Action: "core.jq", Instrument: "transformer"},
				{Name: "b", Action: "core.llm", Instrument: "transformer"},
			},
			want: false,
		},
		{
			name: "unresolvable transformer",
			nodes: []circuit.NodeDef{
				{Name: "a", Action: "unknown.thing", Instrument: "transformer"},
			},
			want: false,
		},
		{
			name: "no handler field — skipped",
			nodes: []circuit.NodeDef{
				{Name: "a"},
				{Name: "b"},
			},
			want: true,
		},
		{
			name:  "empty circuit",
			nodes: nil,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &circuit.CircuitDef{Nodes: tt.nodes}
			got := IsCircuitDeterministic(def, reg)
			if got != tt.want {
				t.Errorf("isCircuitDeterministic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCircuitDeterministic_NilRegistry(t *testing.T) {
	def := &circuit.CircuitDef{Nodes: []circuit.NodeDef{{Name: "a", Action: "core.jq", Instrument: "transformer"}}}
	if IsCircuitDeterministic(def, nil) {
		t.Error("expected false with nil registry")
	}
}

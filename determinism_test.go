package framework

import (
	"context"
	"testing"
)

type deterministicStub struct{ name string }

func (d *deterministicStub) Name() string                                                  { return d.name }
func (d *deterministicStub) Deterministic() bool                                           { return true }
func (d *deterministicStub) Transform(_ context.Context, _ *TransformerContext) (any, error) { return nil, nil }

type stochasticStub struct{ name string }

func (s *stochasticStub) Name() string                                                  { return s.name }
func (s *stochasticStub) Deterministic() bool                                           { return false }
func (s *stochasticStub) Transform(_ context.Context, _ *TransformerContext) (any, error) { return nil, nil }

type unknownStub struct{ name string }

func (u *unknownStub) Name() string                                                  { return u.name }
func (u *unknownStub) Transform(_ context.Context, _ *TransformerContext) (any, error) { return nil, nil }

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
		nodes []NodeDef
		want  bool
	}{
		{
			name: "all deterministic",
			nodes: []NodeDef{
				{Name: "a", Handler: "core.jq", HandlerType: "transformer"},
				{Name: "b", Handler: "core.jq", HandlerType: "transformer"},
			},
			want: true,
		},
		{
			name: "one stochastic",
			nodes: []NodeDef{
				{Name: "a", Handler: "core.jq", HandlerType: "transformer"},
				{Name: "b", Handler: "core.llm", HandlerType: "transformer"},
			},
			want: false,
		},
		{
			name: "unresolvable transformer",
			nodes: []NodeDef{
				{Name: "a", Handler: "unknown.thing", HandlerType: "transformer"},
			},
			want: false,
		},
		{
			name: "no handler field — skipped",
			nodes: []NodeDef{
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
			def := &CircuitDef{Nodes: tt.nodes}
			got := isCircuitDeterministic(def, reg)
			if got != tt.want {
				t.Errorf("isCircuitDeterministic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCircuitDeterministic_NilRegistry(t *testing.T) {
	def := &CircuitDef{Nodes: []NodeDef{{Name: "a", Handler: "core.jq", HandlerType: "transformer"}}}
	if isCircuitDeterministic(def, nil) {
		t.Error("expected false with nil registry")
	}
}

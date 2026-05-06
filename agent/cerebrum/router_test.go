package cerebrum

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

type routerTestCompleter struct {
	name string
}

func (s *routerTestCompleter) Complete(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
	return &tangle.Completion{Content: s.name}, nil
}

func moleculeAtPhase(phase reactivity.AtomType) *reactivity.Molecule {
	m := reactivity.NewMolecule("test")
	m.SetPhase(phase)
	return m
}

func TestSingleRouter(t *testing.T) {
	c := &routerTestCompleter{name: "default"}
	r := SingleRouter(c)

	for _, phase := range reactivity.AllAtomTypes() {
		got := r.Route(moleculeAtPhase(phase))
		if got != c {
			t.Errorf("SingleRouter.Route(%s) returned different completer", phase)
		}
	}
}

func TestPhaseRouter(t *testing.T) {
	fallback := &routerTestCompleter{name: "fallback"}
	think := &routerTestCompleter{name: "think"}
	implement := &routerTestCompleter{name: "implement"}

	r := NewPhaseRouter(fallback)
	r.Set(reactivity.ThinkTriad, think)
	r.Set(reactivity.ImplementTriad, implement)

	tests := []struct {
		phase    reactivity.AtomType
		expected string
	}{
		{reactivity.IntentAtom, "think"},
		{reactivity.AssessmentAtom, "think"},
		{reactivity.KnowledgeAtom, "think"},
		{reactivity.ExpansionAtom, "fallback"},
		{reactivity.SelectionAtom, "fallback"},
		{reactivity.ExecutionAtom, "implement"},
		{reactivity.AcclimationAtom, "implement"},
		{reactivity.RetrospectionAtom, "fallback"},
	}

	for _, tt := range tests {
		got := r.Route(moleculeAtPhase(tt.phase)).(*routerTestCompleter)
		if got.name != tt.expected {
			t.Errorf("Route(%s) = %s, want %s", tt.phase, got.name, tt.expected)
		}
	}
}

func TestAdaptiveRouter_FamiliarUsesFast(t *testing.T) {
	cfg := reactivity.DefaultConfig
	fast := &routerTestCompleter{name: "fast"}
	deep := &routerTestCompleter{name: "deep"}
	r := NewAdaptiveRouter(fast, deep, &cfg)

	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"done": true},
	})
	m.ReportSensor("done", true)

	got := r.Route(m).(*routerTestCompleter)
	if got.name != "fast" {
		t.Errorf("distance=0, should use fast, got %s", got.name)
	}
}

func TestAdaptiveRouter_NovelUsesDeep(t *testing.T) {
	cfg := reactivity.DefaultConfig
	fast := &routerTestCompleter{name: "fast"}
	deep := &routerTestCompleter{name: "deep"}
	r := NewAdaptiveRouter(fast, deep, &cfg)

	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": true, "b": true, "c": true},
	})

	got := r.Route(m).(*routerTestCompleter)
	if got.name != "deep" {
		t.Errorf("distance=1.0, should use deep, got %s", got.name)
	}
}

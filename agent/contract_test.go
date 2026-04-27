package agent

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/agent/corpus"
)

func TestStubReactivityCycle(t *testing.T) {
	r := &StubReactivity{}
	if r.Phase() != Intent {
		t.Errorf("expected Intent, got %s", r.Phase())
	}
	expected := []Phase{Plan, Execute, Assert, Retrospect, Done}
	for _, want := range expected {
		got := r.Advance()
		if got != want {
			t.Errorf("expected %s, got %s", want, got)
		}
	}
}

func TestStubReactivityReset(t *testing.T) {
	r := &StubReactivity{}
	r.Advance()
	r.Reset()
	if r.Phase() != Intent {
		t.Errorf("expected Intent after reset, got %s", r.Phase())
	}
}

func TestStubRunner(t *testing.T) {
	runner := &StubRunner{}
	a := &Agent{
		Identity:   "test",
		Persona:    Worker,
		Corpus:     corpus.New(),
		Reactivity: &StubReactivity{},
	}
	if err := runner.Run(context.Background(), a); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !runner.Executed {
		t.Error("runner should be marked as executed")
	}
	if a.Reactivity.Phase() != Done {
		t.Errorf("expected Done phase, got %s", a.Reactivity.Phase())
	}
}

func TestUniformAXI(t *testing.T) {
	tests := []struct {
		uniform Uniform
		hasAXI  bool
	}{
		{Worker, true},
		{Foreman, false},
		{Director, false},
		{Avatar, false},
	}
	for _, tt := range tests {
		if tt.uniform.HasAXI() != tt.hasAXI {
			t.Errorf("%s.HasAXI() = %v, want %v", tt.uniform, tt.uniform.HasAXI(), tt.hasAXI)
		}
	}
}

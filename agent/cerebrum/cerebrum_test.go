package cerebrum

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestThink_FullLoop(t *testing.T) {
	completer := &stubCompleter{response: "response"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer)

	if err := cb.Think(context.Background(), []byte("clean the room")); err != nil {
		t.Fatalf("Think: %v", err)
	}
	m := cb.Result()

	if !m.Sealed() {
		t.Error("Molecule should be sealed after Think")
	}

	if m.TotalMass() == 0 {
		t.Error("Molecule should have atoms after Think")
	}
}

func TestThink_SealOnCompleterError(t *testing.T) {
	completer := &stubCompleter{err: context.DeadlineExceeded}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer)

	if err := cb.Think(context.Background(), []byte("anything")); err != nil {
		t.Fatalf("Think should not return error (Wish handles it): %v", err)
	}
	m := cb.Result()

	if !m.Sealed() {
		t.Error("Molecule should be sealed on Completer error")
	}

	atoms := m.Atoms(reactivity.RetrospectionAtom)
	found := false
	for _, a := range atoms {
		if a.Taxonomy == "retrospection.wish.completer-error" {
			found = true
		}
	}
	if !found {
		t.Error("expected Wish atom with completer-error taxonomy")
	}
}

func TestThink_MoleculeHasAllPhases(t *testing.T) {
	completer := &stubCompleter{response: "ok"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer)

	cb.Think(context.Background(), []byte("investigate failure"))
	m := cb.Result()

	if m.Mass(reactivity.IntentAtom) == 0 {
		t.Error("expected Intent atoms")
	}
	if m.Mass(reactivity.AssessmentAtom) == 0 {
		t.Error("expected Assessment atoms")
	}
	if m.Mass(reactivity.PlanAtom) == 0 {
		t.Error("expected Plan atoms")
	}
	if m.Mass(reactivity.ExecutionAtom) == 0 {
		t.Error("expected Execution atoms")
	}
	if m.Mass(reactivity.RetrospectionAtom) == 0 {
		t.Error("expected Retrospection atoms")
	}
}

func TestThink_MaxTurnsAbort(t *testing.T) {
	completer := &stubCompleter{response: "stuck"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer, WithMaxTurns(3))

	cb.Think(context.Background(), []byte("impossible"))
	m := cb.Result()

	if !m.Sealed() {
		t.Error("should seal after max turns")
	}

	atoms := m.Atoms(reactivity.RetrospectionAtom)
	found := false
	for _, a := range atoms {
		if a.Taxonomy == "retrospection.wish.max-turns-exceeded" {
			found = true
		}
	}
	if !found {
		t.Error("expected max-turns-exceeded Wish")
	}
}

func TestThink_BackwardCompatible(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer)

	if err := cb.Think(context.Background(), []byte("test")); err != nil {
		t.Fatalf("Think: %v", err)
	}
	m := cb.Result()
	if !m.Sealed() {
		t.Error("should seal")
	}
}

func TestThink_WithMotorBus(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	motor := &stubMotorBus{}
	cb := New(reactor, completer, WithMotor(motor))

	cb.Think(context.Background(), []byte("test"))
	if len(motor.commands) == 0 {
		t.Error("expected Motor Bus to receive wish command")
	}
	if motor.commands[0].Kind != "wish" {
		t.Errorf("expected wish command, got %s", motor.commands[0].Kind)
	}
}

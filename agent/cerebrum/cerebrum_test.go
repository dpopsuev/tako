package cerebrum

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
)

func TestCerebrum_IsOrgan(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	if cb.Name() != "cerebrum" {
		t.Errorf("expected organ name 'cerebrum', got %q", cb.Name())
	}

	if err := cb.Think(context.Background(), []byte("test need")); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Error("Molecule should be sealed after Think")
	}
}

func TestCerebrum_Receive_NonBlocking(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	wire := artifact.Wire{Kind: "cerebrum", Payload: []byte("test")}
	if err := cb.Receive(wire); err != nil {
		t.Fatalf("Receive should not error: %v", err)
	}
}

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
	if m.Mass(reactivity.ExpansionAtom) == 0 {
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

func TestThink_StoreIntegration(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	cb.Think(context.Background(), []byte("first need"))
	cb.Think(context.Background(), []byte("second need"))

	store := cb.Store()
	molecules := store.Molecules()
	if len(molecules) != 2 {
		t.Errorf("expected 2 molecules in store, got %d", len(molecules))
	}

	if store.FocusedID() != "" {
		t.Error("no molecule should be focused after Think completes (parked)")
	}
}

func TestThink_EmissionsDispatchedViaMotor(t *testing.T) {
	completer := &stubCompleter{response: `{"atoms":[{"type":"intent","taxonomy":"intent.goal.test","content":"go"}]}`}
	reactor := reactivity.NewReactor(
		reactivity.WithTriad(reactivity.ThinkTriad, &emittingTriadReactor{}),
	)
	motor := &stubMotorBus{}
	cb := New(reactor, completer, WithMotor(motor))

	cb.Think(context.Background(), []byte("test emission"))

	found := false
	for _, cmd := range motor.commands {
		if cmd.Kind == "instrument" && cmd.Target == "emitted-tool" {
			found = true
		}
	}
	if !found {
		t.Error("expected Motor Bus to receive emission from triad reactor")
	}
}

func TestThink_WithMotorBus(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	motor := &stubMotorBus{}
	cb := New(reactor, completer, WithMotor(motor))

	cb.Think(context.Background(), []byte("test"))
	m := cb.Result()
	if !m.Sealed() {
		t.Error("molecule should be sealed")
	}
}

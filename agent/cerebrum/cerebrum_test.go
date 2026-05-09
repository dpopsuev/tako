package cerebrum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestCerebrum_Think(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	if _, err := cb.Think(context.Background(), reactivity.Catalyst{Need: string("test need")}); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Error("Molecule should be sealed after Think")
	}
}

func TestThink_SealsAndProducesAtoms(t *testing.T) {
	completer := &stubCompleter{response: "response"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer)

	if _, err := cb.Think(context.Background(), reactivity.Catalyst{Need: string("clean the room")}); err != nil {
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

	if _, err := cb.Think(context.Background(), reactivity.Catalyst{Need: string("anything")}); err != nil {
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

func TestThink_ConversationalSeal_SetsResponse(t *testing.T) {
	completer := &stubCompleter{response: "I can help with that"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer, WithMaxTurns(5))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: "help me", Desired: map[string]any{"helped": true}})
	m := cb.Result()

	if !m.Sealed() {
		t.Error("molecule should be sealed")
	}
	if m.Response() == "" {
		t.Error("expected response from speak tool call")
	}
	if m.Response() != "I can help with that" {
		t.Errorf("response = %q, want 'I can help with that'", m.Response())
	}
}

func TestThink_MaxTurnsAbort(t *testing.T) {
	completer := &stubCompleter{response: "stuck"}
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer, WithMaxTurns(3))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: "impossible", Desired: map[string]any{"done": true}})
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



func TestThink_MultipleSessions_StoredInMonolog(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: string("first need")})
	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: string("second need")})

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
	completer := &stubCompleter{toolCalls: []tangle.ToolCall{{
		ID:    "tc-phase",
		Name:  "intent",
		Input: json.RawMessage(`{"taxonomy":"intent.goal.test","content":"go","dimensions":["test"]}`),
	}}}
	reactor := reactivity.NewReactor(
		reactivity.WithTriad(reactivity.ThinkTriad, &emittingTriadReactor{}),
	)
	motor := &stubBus{}
	cb := New(reactor, completer, WithMotor(motor))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: "test emission", Desired: map[string]any{"emitted": true}})

	found := false
	for _, cmd := range motor.Events() {
		if cmd.Kind == "organ" && cmd.Source == "emitted-tool" {
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
	motor := &stubBus{}
	cb := New(reactor, completer, WithMotor(motor))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: string("test")})
	m := cb.Result()
	if !m.Sealed() {
		t.Error("molecule should be sealed")
	}
}

func TestThink_ToolCallDispatchedToMotor(t *testing.T) {
	completer := &stubCompleter{
		response: "looking in the fridge",
		toolCalls: []tangle.ToolCall{
			{ID: "call_1", Name: "look_fridge", Input: json.RawMessage(`{}`)},
		},
	}
	reactor := reactivity.NewReactor()
	motor := &stubBus{}
	cb := New(reactor, completer, WithMotor(motor), WithMaxTurns(3),
		WithTurnTimeout(100*time.Millisecond))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: string("find food")})

	found := false
	for _, evt := range motor.Events() {
		if evt.Kind == "organ" && evt.Source == "look_fridge" {
			found = true
		}
	}
	if !found {
		t.Error("expected ToolCall to dispatch as motor Event with Kind=organ Source=look_fridge")
	}
}

func TestThink_MultipleToolCalls(t *testing.T) {
	completer := &stubCompleter{
		response: "cooking",
		toolCalls: []tangle.ToolCall{
			{ID: "call_1", Name: "turn_on_stove", Input: json.RawMessage(`{}`)},
			{ID: "call_2", Name: "look_fridge", Input: json.RawMessage(`{}`)},
		},
	}
	reactor := reactivity.NewReactor()
	motor := &stubBus{}
	cb := New(reactor, completer, WithMotor(motor), WithMaxTurns(3),
		WithTurnTimeout(100*time.Millisecond))

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: string("cook")})

	names := map[string]bool{}
	for _, evt := range motor.Events() {
		if evt.Kind == "organ" {
			names[evt.Source] = true
		}
	}
	if !names["turn_on_stove"] {
		t.Error("expected turn_on_stove tool call dispatched")
	}
	if !names["look_fridge"] {
		t.Error("expected look_fridge tool call dispatched")
	}
}

func TestBudget_OAEThreshold(t *testing.T) {
	b := Budget{
		MaxTurns:    10,
		TurnTimeout: 30,
		MinOAE:      0.7,
	}
	if b.MinOAE != 0.7 {
		t.Errorf("expected MinOAE 0.7, got %f", b.MinOAE)
	}
}

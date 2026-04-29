package cerebrum

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
)

func TestThink_FullLoop(t *testing.T) {
	completer := &instrument.StubCompleter{Response: []byte("response")}
	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)

	m, err := cb.Think(context.Background(), []byte("clean the room"))
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	if !m.Sealed() {
		t.Error("Molecule should be sealed after Think")
	}

	if m.TotalMass() == 0 {
		t.Error("Molecule should have atoms after Think")
	}
}

func TestThink_SealOnCompleterError(t *testing.T) {
	completer := &instrument.StubCompleter{Err: context.DeadlineExceeded}
	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)

	m, err := cb.Think(context.Background(), []byte("anything"))
	if err != nil {
		t.Fatalf("Think should not return error (Wish handles it): %v", err)
	}

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
	completer := &instrument.StubCompleter{Response: []byte("ok")}
	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)

	m, _ := cb.Think(context.Background(), []byte("investigate failure"))

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
	completer := &instrument.StubCompleter{Response: []byte("stuck")}
	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)
	cb.maxTurns = 3

	m, _ := cb.Think(context.Background(), []byte("impossible"))

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

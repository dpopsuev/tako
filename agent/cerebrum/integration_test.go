package cerebrum

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestThink_FullVerticalSlice(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	motor := &stubMotorBus{}
	cb := New(reactor, completer, WithMotor(motor))

	if err := cb.Think(context.Background(), []byte("investigate PTP failure")); err != nil {
		t.Fatalf("Think: %v", err)
	}
	m := cb.Result()

	if !m.Sealed() {
		t.Fatal("Molecule should be sealed")
	}

	if m.Mass(reactivity.IntentAtom) == 0 {
		t.Error("missing Intent atoms")
	}
	if m.Mass(reactivity.RetrospectionAtom) == 0 {
		t.Error("missing Retrospection atoms (Wish)")
	}

	if len(motor.commands) == 0 {
		t.Error("Motor Bus should have received wish command")
	}

	t.Logf("Vertical slice: %d atoms, %d motor commands", m.TotalMass(), len(motor.commands))
}

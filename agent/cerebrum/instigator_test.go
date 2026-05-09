package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestInstigator_HelloSkipsToReflect(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired: false,
		SenseCount: 0,
		MotorCount: 0,
	}

	next := ins.NextTriad(reactivity.ThinkTriad, ctx)
	if next != reactivity.ReflectTriad {
		t.Errorf("no Desired should skip to reflect, got %v", next)
	}
}

func TestInstigator_WithDesiredAndKnowledge_GoesToImplement(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired:    true,
		MassKnowledge: 1,
		SenseCount:    1,
	}

	next := ins.NextTriad(reactivity.ThinkTriad, ctx)
	if next != reactivity.ImplementTriad {
		t.Errorf("Desired + knowledge should go to implement, got %v", next)
	}
}

func TestInstigator_WithDesiredNoKnowledge_GoesToCompose(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired: true,
		SenseCount: 1,
	}

	next := ins.NextTriad(reactivity.ThinkTriad, ctx)
	if next != reactivity.ComposeTriad {
		t.Errorf("Desired + senses but no knowledge should go to compose, got %v", next)
	}
}

func TestInstigator_ComposeToImplement(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired:     true,
		MassAssessment: 1,
	}

	next := ins.NextTriad(reactivity.ComposeTriad, ctx)
	if next != reactivity.ImplementTriad {
		t.Errorf("assessment done should go to implement, got %v", next)
	}
}

func TestInstigator_ImplementToReflect_TelosReached(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired:      true,
		Distance:        0,
		HasVerification: true,
	}

	next := ins.NextTriad(reactivity.ImplementTriad, ctx)
	if next != reactivity.ReflectTriad {
		t.Errorf("Telos reached should go to reflect, got %v", next)
	}
}

func TestInstigator_ImplementStays_NotVerified(t *testing.T) {
	ins := MustInstigator(nil)
	ctx := InstigatorContext{
		HasDesired:      true,
		Distance:        0.5,
		HasVerification: false,
		MotorCount:      1,
	}

	next := ins.NextTriad(reactivity.ImplementTriad, ctx)
	if next != reactivity.ImplementTriad {
		t.Errorf("not verified should stay in implement, got %v", next)
	}
}

func TestInstigator_CustomContracts(t *testing.T) {
	contracts := []struct{ From, To, Predicate string }{
		{"think", "reflect", "TurnCount > 3"},
	}
	ins := MustInstigator(contracts)
	ctx := InstigatorContext{TurnCount: 5}

	next := ins.NextTriad(reactivity.ThinkTriad, ctx)
	if next != reactivity.ReflectTriad {
		t.Errorf("custom contract should fire, got %v", next)
	}
}

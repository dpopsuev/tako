package fab

import (
	"errors"
	"testing"

	"github.com/dpopsuev/tako/artifact"
)

func TestContractAlwaysPass(t *testing.T) {
	evaluator := AlwaysPass{}
	contract := Contract{From: "a", To: "b", Evaluator: evaluator}
	envelope := artifact.NewEnvelope("a", []byte("test"))
	ok, err := evaluator.Evaluate(contract, envelope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("AlwaysPass should return true")
	}
}

func TestAssemblyIntake(t *testing.T) {
	a := StubAssembly()
	intake, err := a.Intake()
	if err != nil {
		t.Fatalf("Intake() failed: %v", err)
	}
	if intake.Name != "intake" {
		t.Errorf("expected intake station, got %s", intake.Name)
	}
	if !intake.Intake {
		t.Error("intake station should have Intake=true")
	}
}

func TestAssemblySuccessors(t *testing.T) {
	a := StubAssembly()
	successors := a.Successors("intake")
	if len(successors) != 1 || successors[0] != "terminus" {
		t.Errorf("expected [terminus], got %v", successors)
	}
}

func TestAssemblyNoIntake(t *testing.T) {
	a := Assembly{Name: "empty", Stations: map[string]Station{}}
	_, err := a.Intake()
	if !errors.Is(err, ErrNoIntake) {
		t.Errorf("expected ErrNoIntake, got %v", err)
	}
}

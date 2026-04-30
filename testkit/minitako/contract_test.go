package minitako

import (
	"testing"

	"github.com/dpopsuev/tako/fab"
)

func TestDayDone_PassesOnTickerZero(t *testing.T) {
	gs := NewGameState()
	gs.ActionTicker = 0
	env := PackState(&gs)

	ok, err := DayDone.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("should pass when ticker is 0")
	}
}

func TestDayDone_FailsWhenTickerRemains(t *testing.T) {
	gs := NewGameState()
	gs.ActionTicker = 5
	env := PackState(&gs)

	ok, err := DayDone.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("should not pass when ticker > 0")
	}
}

func TestPetDied_PassesOnDeath(t *testing.T) {
	gs := NewGameState()
	gs.Alive = false
	env := PackState(&gs)

	ok, err := PetDied.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("should pass when pet is dead")
	}
}

func TestPetDied_FailsWhenAlive(t *testing.T) {
	gs := NewGameState()
	env := PackState(&gs)

	ok, err := PetDied.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("should not pass when pet is alive")
	}
}

func TestPackUnpack_RoundTrip(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 42
	gs.Wallet = 100
	gs.Stage = Pup
	gs.Day = 3

	env := PackState(&gs)
	if !env.Verify() {
		t.Error("envelope should verify after seal")
	}

	unpacked, err := UnpackState(env)
	if err != nil {
		t.Fatalf("UnpackState: %v", err)
	}
	if unpacked.Pet.Hunger != 42 {
		t.Errorf("hunger mismatch: %d", unpacked.Pet.Hunger)
	}
	if unpacked.Wallet != 100 {
		t.Errorf("wallet mismatch: %d", unpacked.Wallet)
	}
	if unpacked.Stage != Pup {
		t.Errorf("stage mismatch: %s", unpacked.Stage)
	}
}

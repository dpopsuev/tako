package minitako

import (
	"testing"

	"github.com/dpopsuev/tako/fab"
)

func TestDayContract_NotDoneBeforeMax(t *testing.T) {
	gs := NewGameState()
	gs.ActionTicker = 5
	env := PackState(&gs)

	dc := NewDayContract()
	done, err := dc.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if done {
		t.Error("should not be done at 5 actions")
	}
}

func TestDayContract_DoneAtMax(t *testing.T) {
	gs := NewGameState()
	gs.ActionTicker = 14
	env := PackState(&gs)

	dc := NewDayContract()
	done, err := dc.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !done {
		t.Error("should be done at 14 actions")
	}
}

func TestNightContract_AlwaysDone(t *testing.T) {
	env := PackState(&GameState{})
	nc := NewNightContract()
	done, err := nc.Evaluate(fab.Contract{}, env)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !done {
		t.Error("night contract should always pass")
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

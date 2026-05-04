package minitako

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/tako/agent/shell"
)

func TestAction_Feed_IncreasesHunger(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 50
	_, err := ApplyAction(&gs, Feed)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if gs.Pet.Hunger <= 50 {
		t.Errorf("hunger should increase after feed, got %d", gs.Pet.Hunger)
	}
}

func TestAction_Feed_ScalesWithGrowth(t *testing.T) {
	egg := NewGameState()
	egg.Pet.Hunger = 50
	kraken := NewGameState()
	kraken.Pet.Hunger = 50
	kraken.Stage = Kraken

	ApplyAction(&egg, Feed)
	ApplyAction(&kraken, Feed)

	if kraken.Pet.Hunger >= egg.Pet.Hunger {
		t.Errorf("kraken feed (%d) should be less effective than egg feed (%d)", kraken.Pet.Hunger, egg.Pet.Hunger)
	}
}

func TestAction_Feed_DiminishingReturns(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 20
	ApplyAction(&gs, Feed)
	first := gs.Pet.Hunger

	gs.Pet.Hunger = 20
	ApplyAction(&gs, Feed)

	gs.Pet.Hunger = 20
	ApplyAction(&gs, Feed)
	third := gs.Pet.Hunger

	if third >= first {
		t.Errorf("3rd consecutive feed (%d) should give less than 1st (%d)", third-20, first-20)
	}
}

func TestAction_Hunt_RequiresRifle(t *testing.T) {
	gs := NewGameState()
	gs.Stage = Adolescent
	_, err := ApplyAction(&gs, Hunt)
	if err != ErrNoRifle {
		t.Errorf("expected ErrNoRifle, got %v", err)
	}
}

func TestAction_Hunt_RequiresAmmo(t *testing.T) {
	gs := NewGameState()
	gs.Stage = Adolescent
	gs.HasRifle = true
	gs.Ammo = 0
	_, err := ApplyAction(&gs, Hunt)
	if err != ErrNoAmmo {
		t.Errorf("expected ErrNoAmmo, got %v", err)
	}
}

func TestAction_Hunt_Success(t *testing.T) {
	gs := NewGameState()
	gs.Stage = Adolescent
	gs.HasRifle = true
	gs.Ammo = 3
	gs.Pet.Hunger = 30

	_, err := ApplyAction(&gs, Hunt)
	if err != nil {
		t.Fatalf("Hunt: %v", err)
	}
	if gs.Pet.Hunger <= 30 {
		t.Errorf("hunger should increase after hunt, got %d", gs.Pet.Hunger)
	}
	if gs.Ammo != 2 {
		t.Errorf("ammo should decrease, got %d", gs.Ammo)
	}
}

func TestAction_Medicine_OnlyWhenSick(t *testing.T) {
	gs := NewGameState()
	_, err := ApplyAction(&gs, Medicine)
	if err != ErrNotSick {
		t.Errorf("expected ErrNotSick for healthy pet, got %v", err)
	}
}

func TestAction_Work_EarnsCoins(t *testing.T) {
	gs := NewGameState()
	ApplyAction(&gs, Work)
	if gs.Wallet <= 0 {
		t.Errorf("wallet should increase after work, got %d", gs.Wallet)
	}
}

func TestAction_NightBlocked(t *testing.T) {
	gs := NewGameState()
	gs.Hour = 23
	_, err := ApplyAction(&gs, Feed)
	if err != ErrNightTime {
		t.Errorf("expected ErrNightTime, got %v", err)
	}
}

func TestAvailableActions_EggBasic(t *testing.T) {
	gs := NewGameState()
	actions := AvailableActions(&gs)

	has := func(a Action) bool {
		for _, act := range actions {
			if act == a {
				return true
			}
		}
		return false
	}

	if !has(Feed) {
		t.Error("Egg should have Feed")
	}
	if has(Hunt) {
		t.Error("Egg should not have Hunt")
	}
	if has(Wrestle) {
		t.Error("Egg should not have Wrestle")
	}
}

func TestAvailableActions_AdolescentUnlocksHunt(t *testing.T) {
	gs := NewGameState()
	gs.Stage = Adolescent
	actions := AvailableActions(&gs)

	for _, a := range actions {
		if a == Hunt {
			return
		}
	}
	t.Error("Adolescent should have Hunt")
}

func TestActionFunction_SatisfiesInterface(t *testing.T) {
	gs := NewGameState()
	var _ shell.Function = NewActionFunction(Feed, &gs)
}

func TestActionFunction_Execute(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 50
	fn := NewActionFunction(Feed, &gs)

	result, err := fn.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	if gs.Pet.Hunger <= 50 {
		t.Errorf("hunger should increase, got %d", gs.Pet.Hunger)
	}
}

func TestGameShell_Names(t *testing.T) {
	gs := NewGameState()
	shell := GameShell(&gs)

	names := shell.Names()
	if len(names) == 0 {
		t.Error("shell should have instrument names")
	}
}

func TestGameShell_Describe(t *testing.T) {
	gs := NewGameState()
	shell := GameShell(&gs)

	desc, err := shell.Describe("feed")
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if desc == "" {
		t.Error("description should not be empty")
	}
}

func TestGameShell_Schema(t *testing.T) {
	gs := NewGameState()
	shell := GameShell(&gs)

	schema, err := shell.Schema("feed")
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if !json.Valid(schema) {
		t.Error("schema should be valid JSON")
	}
}

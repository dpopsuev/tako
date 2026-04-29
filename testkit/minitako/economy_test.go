package minitako

import "testing"

func TestBuy_DeductsWallet(t *testing.T) {
	gs := NewGameState()
	gs.Wallet = 100
	err := Buy(&gs, BabyFood)
	if err != nil {
		t.Fatalf("Buy: %v", err)
	}
	if gs.Wallet != 95 {
		t.Errorf("wallet should be 95, got %d", gs.Wallet)
	}
}

func TestBuy_InsufficientFunds(t *testing.T) {
	gs := NewGameState()
	gs.Wallet = 3
	err := Buy(&gs, BabyFood)
	if err != ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestBuy_Rifle(t *testing.T) {
	gs := NewGameState()
	gs.Wallet = 200
	err := Buy(&gs, Rifle)
	if err != nil {
		t.Fatalf("Buy rifle: %v", err)
	}
	if !gs.HasRifle {
		t.Error("should have rifle after purchase")
	}
	if gs.Wallet != 0 {
		t.Errorf("wallet should be 0, got %d", gs.Wallet)
	}
}

func TestBuy_Ammo(t *testing.T) {
	gs := NewGameState()
	gs.Wallet = 20
	err := Buy(&gs, AmmoBox)
	if err != nil {
		t.Fatalf("Buy ammo: %v", err)
	}
	if gs.Ammo != 5 {
		t.Errorf("ammo should be 5, got %d", gs.Ammo)
	}
}

func TestSitter_CheapAlwaysFeeds(t *testing.T) {
	gs := NewGameState()
	gs.Sitter = CheapSitter
	action := SitterAct(&gs)
	if action != Feed {
		t.Errorf("cheap sitter should feed, got %s", action)
	}
}

func TestSitter_StandardPicksLowest(t *testing.T) {
	gs := NewGameState()
	gs.Sitter = StandardSitter
	gs.Pet.Hygiene = 10
	action := SitterAct(&gs)
	if action != Clean {
		t.Errorf("standard sitter should clean lowest stat, got %s", action)
	}
}

func TestSitter_NoSitterIdles(t *testing.T) {
	gs := NewGameState()
	gs.Sitter = NoSitter
	action := SitterAct(&gs)
	if action != Idle {
		t.Errorf("no sitter should idle, got %s", action)
	}
}

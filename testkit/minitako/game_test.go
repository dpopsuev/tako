package minitako

import "testing"

func TestRunGame_RandomPlayerSurvives(t *testing.T) {
	result := RunGame(RandomPlayer{}, StubInspector{})

	t.Logf("Random player: survived %d ticks, peak=%s, died=%s, OAE=%.2f",
		result.TicksSurvived, result.PeakStage, result.DeathCause, result.OAE)

	if result.TicksSurvived == 0 {
		t.Error("random player should survive at least 1 tick")
	}
}

func TestRunGame_DeathReturnsResult(t *testing.T) {
	result := RunGame(RandomPlayer{}, StubInspector{})

	if result.DeathCause == "" && result.DaysSurvived <= 7 {
		t.Error("should have death cause or survive all 7 days")
	}
}

func TestRunGame_OAEProduced(t *testing.T) {
	result := RunGame(RandomPlayer{}, StubInspector{})

	if result.OAE < 0 {
		t.Errorf("OAE should be non-negative, got %f", result.OAE)
	}
	t.Logf("OAE: %.2f", result.OAE)
}

func TestRunGame_DayNightCycle(t *testing.T) {
	result := RunGame(RandomPlayer{}, StubInspector{})

	if result.DaysSurvived < 1 {
		t.Error("should survive at least day 1")
	}
	t.Logf("Survived %d days", result.DaysSurvived)
}

func TestArcadeAssembly_HasStation(t *testing.T) {
	asm := ArcadeAssembly()
	station, err := asm.Intake()
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if station.Name != "arcade" {
		t.Errorf("expected arcade, got %s", station.Name)
	}
	if len(station.Instruments) == 0 {
		t.Error("station should have instruments")
	}
}

func TestArcadeAssembly_HasContract(t *testing.T) {
	asm := ArcadeAssembly()
	contracts := asm.ContractsFrom("arcade")
	if len(contracts) == 0 {
		t.Error("should have contracts from arcade")
	}
}

func TestSeedArtifact_RoundTrips(t *testing.T) {
	env := SeedArtifact()
	gs, err := UnpackState(env)
	if err != nil {
		t.Fatalf("UnpackState: %v", err)
	}
	if gs.ActionTicker != 14 {
		t.Errorf("ticker should be 14, got %d", gs.ActionTicker)
	}
	if !gs.Alive {
		t.Error("should be alive")
	}
}

func TestNightTransition_AdvancesDay(t *testing.T) {
	gs := NewGameState()
	gs.ActionTicker = 0
	gs.Day = 1
	env := PackState(&gs)

	env, err := NightTransition(env)
	if err != nil {
		t.Fatalf("NightTransition: %v", err)
	}

	gs2, _ := UnpackState(env)
	if gs2.Day != 2 {
		t.Errorf("day should advance to 2, got %d", gs2.Day)
	}
	if gs2.ActionTicker != 14 {
		t.Errorf("ticker should reset to 14, got %d", gs2.ActionTicker)
	}
	if gs2.Hour != 6 {
		t.Errorf("hour should be 6 (dawn), got %d", gs2.Hour)
	}
}


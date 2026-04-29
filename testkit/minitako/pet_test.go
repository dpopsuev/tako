package minitako

import "testing"

func TestPet_Decay_EggBaseline(t *testing.T) {
	gs := NewGameState()
	pet := Pet{}
	pet.Decay(&gs)

	if gs.Pet.Hunger >= 100 {
		t.Error("hunger should decay")
	}
	if gs.Pet.Energy >= 100 {
		t.Error("energy should decay")
	}
	if gs.Pet.Happiness >= 100 {
		t.Error("happiness should decay")
	}
	if gs.Pet.Hygiene >= 100 {
		t.Error("hygiene should decay")
	}
}

func TestPet_Decay_KrakenFaster(t *testing.T) {
	egg := NewGameState()
	kraken := NewGameState()
	kraken.Stage = Kraken
	pet := Pet{}

	pet.Decay(&egg)
	pet.Decay(&kraken)

	if kraken.Pet.Hunger >= egg.Pet.Hunger {
		t.Errorf("kraken hunger %d should decay faster than egg %d", kraken.Pet.Hunger, egg.Pet.Hunger)
	}
}

func TestPet_NightDecay_HalvedButEnergyRestores(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Energy = 50
	pet := Pet{}
	pet.NightDecay(&gs)

	if gs.Pet.Energy <= 50 {
		t.Errorf("energy should restore at night, got %d", gs.Pet.Energy)
	}
	if gs.Pet.Hunger >= 100 {
		t.Error("hunger should still decay at night")
	}
}

func TestPet_Cascade_HungryAndTired(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 15
	gs.Pet.Energy = 20
	healthBefore := gs.Pet.Health
	pet := Pet{}
	pet.Cascade(&gs)

	if gs.Pet.Health >= healthBefore {
		t.Errorf("health should drop from hungry+tired cascade, was %d now %d", healthBefore, gs.Pet.Health)
	}
}

func TestPet_Cascade_SickAndDirty(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Health = 30
	gs.Pet.Hygiene = 20
	healthBefore := gs.Pet.Health
	pet := Pet{}
	pet.Cascade(&gs)

	if gs.Pet.Health >= healthBefore {
		t.Errorf("health should drop from sick+dirty cascade, was %d now %d", healthBefore, gs.Pet.Health)
	}
}

func TestPet_Death_Starvation(t *testing.T) {
	gs := NewGameState()
	gs.Pet.Hunger = 0
	pet := Pet{}

	dead, cause := pet.CheckDeath(&gs)
	if !dead {
		t.Error("should be dead")
	}
	if cause != "starvation" {
		t.Errorf("cause should be starvation, got %s", cause)
	}
}

func TestPet_Death_Alive(t *testing.T) {
	gs := NewGameState()
	pet := Pet{}

	dead, _ := pet.CheckDeath(&gs)
	if dead {
		t.Error("should be alive")
	}
}

func TestPet_Growth_StageTransitions(t *testing.T) {
	gs := NewGameState()
	pet := Pet{}

	for i := 0; i < 24; i++ {
		pet.Grow(&gs)
	}
	if gs.Stage != Hatchling {
		t.Errorf("expected Hatchling at age 24, got %s", gs.Stage)
	}

	for i := 0; i < 24; i++ {
		pet.Grow(&gs)
	}
	if gs.Stage != Pup {
		t.Errorf("expected Pup at age 48, got %s", gs.Stage)
	}
}

func TestStageForAge_AllStages(t *testing.T) {
	tests := []struct {
		age  int
		want GrowthStage
	}{
		{0, Egg}, {23, Egg},
		{24, Hatchling}, {47, Hatchling},
		{48, Pup}, {71, Pup},
		{72, Juvenile}, {95, Juvenile},
		{96, Adolescent}, {119, Adolescent},
		{120, Adult}, {143, Adult},
		{144, Kraken}, {200, Kraken},
	}
	for _, tt := range tests {
		got := StageForAge(tt.age)
		if got != tt.want {
			t.Errorf("StageForAge(%d) = %s, want %s", tt.age, got, tt.want)
		}
	}
}

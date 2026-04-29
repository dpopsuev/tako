package minitako

import "math"

type Pet struct{}

func (p Pet) Decay(gs *GameState) {
	mult := gs.Stage.DecayMultiplier()
	gs.Pet.Hunger -= int(math.Ceil(2.0 * mult))
	gs.Pet.Energy -= int(math.Ceil(1.0 * mult))
	gs.Pet.Happiness -= int(math.Ceil(1.0 * mult))
	gs.Pet.Health -= int(math.Ceil(0.5 * mult))
	gs.Pet.Hygiene -= int(math.Ceil(1.5 * mult))
	p.clamp(&gs.Pet)
}

func (p Pet) NightDecay(gs *GameState) {
	mult := gs.Stage.DecayMultiplier() * 0.5
	gs.Pet.Hunger -= int(math.Ceil(2.0 * mult))
	gs.Pet.Happiness -= int(math.Ceil(1.0 * mult))
	gs.Pet.Health -= int(math.Ceil(0.5 * mult))
	gs.Pet.Hygiene -= int(math.Ceil(1.5 * mult))
	gs.Pet.Energy += 3
	p.clamp(&gs.Pet)
}

func (p Pet) Cascade(gs *GameState) {
	hungry := gs.Pet.Hunger < 20
	tired := gs.Pet.Energy < 25
	sick := gs.Pet.Health < 40
	dirty := gs.Pet.Hygiene < 35
	depressed := gs.Pet.Happiness < 30

	if hungry && tired {
		gs.Pet.Health -= int(math.Ceil(0.5 * gs.Stage.DecayMultiplier() * 2))
	}
	if sick && dirty {
		gs.Pet.Health -= int(math.Ceil(0.5 * gs.Stage.DecayMultiplier() * 4))
	}
	if depressed && hungry {
		gs.Pet.Energy -= int(math.Ceil(1.0 * gs.Stage.DecayMultiplier() * 2))
	}

	allLow := gs.Pet.Hunger < 30 && gs.Pet.Energy < 30 &&
		gs.Pet.Happiness < 30 && gs.Pet.Health < 30 && gs.Pet.Hygiene < 30
	if allLow {
		gs.Pet.Hunger -= int(math.Ceil(2.0 * gs.Stage.DecayMultiplier()))
		gs.Pet.Energy -= int(math.Ceil(1.0 * gs.Stage.DecayMultiplier()))
		gs.Pet.Happiness -= int(math.Ceil(1.0 * gs.Stage.DecayMultiplier()))
		gs.Pet.Health -= int(math.Ceil(0.5 * gs.Stage.DecayMultiplier()))
		gs.Pet.Hygiene -= int(math.Ceil(1.5 * gs.Stage.DecayMultiplier()))
	}
	p.clamp(&gs.Pet)
}

func (p Pet) CheckDeath(gs *GameState) (bool, string) {
	switch {
	case gs.Pet.Hunger <= 0:
		return true, "starvation"
	case gs.Pet.Energy <= 0:
		return true, "exhaustion"
	case gs.Pet.Health <= 0:
		return true, "illness"
	default:
		return false, ""
	}
}

func (p Pet) Grow(gs *GameState) {
	gs.Age++
	gs.Stage = StageForAge(gs.Age)
}

func (p Pet) clamp(stats *PetStats) {
	stats.Hunger = clampStat(stats.Hunger)
	stats.Energy = clampStat(stats.Energy)
	stats.Happiness = clampStat(stats.Happiness)
	stats.Health = clampStat(stats.Health)
	stats.Hygiene = clampStat(stats.Hygiene)
}

func clampStat(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

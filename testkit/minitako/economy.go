package minitako

type StoreItem int

const (
	BabyFood StoreItem = iota
	Kibble
	MatureFood
	RawMeat
	BasicMedicine
	Vitamins
	Rifle
	AmmoBox
)

func (s StoreItem) Price() int {
	return [...]int{5, 15, 40, 80, 10, 25, 200, 20}[s]
}

func (s StoreItem) String() string {
	return [...]string{
		"baby-food", "kibble", "mature-food", "raw-meat",
		"basic-medicine", "vitamins", "rifle", "ammo",
	}[s]
}

func Buy(gs *GameState, item StoreItem) error {
	price := item.Price()
	if gs.Wallet < price {
		return ErrInsufficientFunds
	}
	gs.Wallet -= price

	switch item {
	case BabyFood:
		gs.Pet.Hunger += 30
	case Kibble:
		gs.Pet.Hunger += 25
	case MatureFood:
		gs.Pet.Hunger += 20
	case RawMeat:
		gs.Pet.Hunger += 15
	case BasicMedicine:
		gs.Pet.Health += 20
	case Vitamins:
		gs.Pet.Hunger += 5
		gs.Pet.Energy += 5
		gs.Pet.Happiness += 5
		gs.Pet.Health += 5
		gs.Pet.Hygiene += 5
	case Rifle:
		gs.HasRifle = true
	case AmmoBox:
		gs.Ammo += 5
	}

	pet := Pet{}
	pet.clamp(&gs.Pet)
	return nil
}

func HireSitter(gs *GameState, tier SitterTier) {
	gs.Sitter = tier
}

func SitterAct(gs *GameState) Action {
	switch gs.Sitter {
	case CheapSitter:
		return cheapSitterPick(gs)
	case StandardSitter:
		return standardSitterPick(gs)
	case PremiumSitter:
		return premiumSitterPick(gs)
	default:
		return Idle
	}
}

func cheapSitterPick(_ *GameState) Action {
	return Feed
}

func standardSitterPick(gs *GameState) Action {
	lowest := gs.Pet.Hunger
	pick := Feed
	if gs.Pet.Energy < lowest {
		lowest = gs.Pet.Energy
		pick = Rest
	}
	if gs.Pet.Happiness < lowest {
		lowest = gs.Pet.Happiness
		pick = Play
	}
	if gs.Pet.Hygiene < lowest {
		pick = Clean
	}
	return pick
}

func premiumSitterPick(gs *GameState) Action {
	return standardSitterPick(gs)
}

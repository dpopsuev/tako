package minitako

import "time"

type Action int

const (
	Feed Action = iota
	Rest
	Play
	Clean
	Medicine
	Comfort
	Hunt
	Patrol
	Wrestle
	Work
	Shop
	Browse
	Idle
)

func (a Action) String() string {
	return [...]string{
		"feed", "rest", "play", "clean", "medicine", "comfort",
		"hunt", "patrol", "wrestle", "work", "shop", "browse", "idle",
	}[a]
}

type GrowthStage int

const (
	Egg GrowthStage = iota
	Hatchling
	Pup
	Juvenile
	Adolescent
	Adult
	Kraken
)

func (g GrowthStage) String() string {
	return [...]string{"egg", "hatchling", "pup", "juvenile", "adolescent", "adult", "kraken"}[g]
}

func (g GrowthStage) DecayMultiplier() float64 {
	return [...]float64{1.0, 1.3, 1.7, 2.2, 3.0, 4.0, 6.0}[g]
}

func (g GrowthStage) FeedEffectiveness() int {
	return [...]int{30, 30, 25, 25, 20, 15, 10}[g]
}

func StageForAge(age int) GrowthStage {
	switch {
	case age < 24:
		return Egg
	case age < 48:
		return Hatchling
	case age < 72:
		return Pup
	case age < 96:
		return Juvenile
	case age < 120:
		return Adolescent
	case age < 144:
		return Adult
	default:
		return Kraken
	}
}

type TimeOfDay int

const (
	Night TimeOfDay = iota
	Dawn
	Day
	Dusk
)

func HourToTimeOfDay(hour int) TimeOfDay {
	switch {
	case hour == 6:
		return Dawn
	case hour > 6 && hour < 20:
		return Day
	case hour == 20:
		return Dusk
	default:
		return Night
	}
}

type SitterTier int

const (
	NoSitter SitterTier = iota
	CheapSitter
	StandardSitter
	PremiumSitter
)

func (s SitterTier) CostPerTick() int {
	return [...]int{0, 5, 15, 30}[s]
}

type PetStats struct {
	Hunger    int
	Energy    int
	Happiness int
	Health    int
	Hygiene   int
}

type GameState struct {
	Pet           PetStats
	Alive         bool
	Age           int
	Stage         GrowthStage
	Wallet        int
	HasRifle      bool
	Ammo          int
	ActionTicker  int
	Hour          int
	Day           int
	Sitter        SitterTier
	LastAction    Action
	ConsecutiveOf int
	GuidesRead    map[string]bool
	CreatedAt     time.Time
}

func NewGameState() GameState {
	return GameState{
		Pet: PetStats{
			Hunger:    100,
			Energy:    100,
			Happiness: 100,
			Health:    100,
			Hygiene:   100,
		},
		Alive:      true,
		Stage:      Egg,
		Wallet:     0,
		Hour:       6,
		Day:        1,
		GuidesRead: make(map[string]bool),
		CreatedAt:  time.Now(),
	}
}

type Need struct {
	Stat    string
	Value   int
	Urgency float64
}

type RunResult struct {
	PeakAge      int
	PeakStage    GrowthStage
	DaysSurvived int
	TicksSurvived int
	OAE          float64
	CoinsEarned  int
	CoinsSpent   int
	WishCount    int
	DeathCause   string
}

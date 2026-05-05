package rehearsal

import "time"

type Template int

const (
	ReadOnly  Template = iota
	Write
	MultiTurn
)

func (t Template) String() string {
	switch t {
	case ReadOnly:
		return "read-only"
	case Write:
		return "write"
	case MultiTurn:
		return "multi-turn"
	default:
		return "unknown"
	}
}

type Scale int

const (
	Single   Scale = 1
	Pair     Scale = 2
	Fireteam Scale = 4
	Squad    Scale = 10
	Platoon  Scale = 30
)

func (s Scale) String() string {
	switch s {
	case Single:
		return "single"
	case Pair:
		return "pair"
	case Fireteam:
		return "fireteam"
	case Squad:
		return "squad"
	case Platoon:
		return "platoon"
	default:
		return "unknown"
	}
}

func (s Scale) Implemented() bool {
	return s == Single
}

type Rehearsal struct {
	Name       string
	Scale      Scale
	Template   Template
	Prompts    []string
	Setup      []SetupOption
	ExtraRules []Rule
	MustUse    []string
	MustNotUse []string
	ExpectFile string
	Timeout    time.Duration
}

func (r Rehearsal) TimeoutOrDefault() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return 120 * time.Second
}

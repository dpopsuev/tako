package minitako

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dpopsuev/tako/agent/organ"
)

var (
	ErrNoRifle           = errors.New("minitako: hunt requires rifle")
	ErrNoAmmo            = errors.New("minitako: hunt requires ammo")
	ErrInsufficientFunds = errors.New("minitako: insufficient funds")
	ErrNotSick           = errors.New("minitako: medicine only works when sick")
	ErrNightTime         = errors.New("minitako: action unavailable at night")
	ErrUnknownAction     = errors.New("minitako: unknown action")
)

type ActionFunction struct {
	action Action
	gs     *GameState
}



func NewActionFunction(a Action, gs *GameState) *ActionFunction {
	return &ActionFunction{action: a, gs: gs}
}

func (f *ActionFunction) Name() string        { return f.action.String() }
func (f *ActionFunction) Description() string { return descriptions[f.action] }
func (f *ActionFunction) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (f *ActionFunction) Execute(_ context.Context, _ json.RawMessage) (organ.Result, error) {
	return ApplyAction(f.gs, f.action)
}

var descriptions = map[Action]string{
	Feed:     "Feed the pet. Hunger +30 (scaled by growth). Energy -5.",
	Rest:     "Rest. Energy +25. Takes 2 ticks. Hunger -10.",
	Play:     "Play with the pet. Happiness +20. Energy -10, Hunger -5.",
	Clean:    "Clean the pet. Hygiene +35. Happiness -5.",
	Medicine: "Give medicine. Health +20. Happiness -10. Only works when sick.",
	Comfort:  "Comfort the pet. Happiness +10, Health +5. No side effects.",
	Hunt:     "Hunt big game. Hunger +50. Requires rifle + ammo. Energy -20.",
	Wrestle:  "Wrestle. Happiness +30. Energy -25, Hunger -15.",
	Patrol:   "Patrol territory. Energy -15.",
	Work:     "Work at factory. Earn coins. Pet unattended unless sitter hired.",
	Shop:     "Visit the store.",
	Browse:   "Read a forum guide. Returns guide text.",
	Idle:     "Do nothing.",
}

func ApplyAction(gs *GameState, action Action) (organ.Result, error) {
	if HourToTimeOfDay(gs.Hour) == Night {
		return organ.Result{}, ErrNightTime
	}

	consecutive := 1
	if gs.LastAction == action {
		consecutive = gs.ConsecutiveOf + 1
	}

	var err error

	switch action {
	case Feed:
		eff := gs.Stage.FeedEffectiveness()
		if consecutive >= 3 {
			eff /= 2
		}
		gs.Pet.Hunger += eff
		gs.Pet.Energy -= 5

	case Rest:
		gs.Pet.Energy += 25
		gs.Pet.Hunger -= 10

	case Play:
		bonus := 20
		if gs.Stage >= Kraken {
			bonus = 5
		}
		if consecutive >= 3 {
			bonus /= 2
		}
		gs.Pet.Happiness += bonus
		gs.Pet.Energy -= 10
		gs.Pet.Hunger -= 5

	case Clean:
		gs.Pet.Hygiene += 35
		gs.Pet.Happiness -= 5

	case Medicine:
		if gs.Pet.Health >= 40 {
			return organ.Result{}, ErrNotSick
		}
		gs.Pet.Health += 20
		gs.Pet.Happiness -= 10

	case Comfort:
		gs.Pet.Happiness += 10
		gs.Pet.Health += 5

	case Hunt:
		if !gs.HasRifle {
			return organ.Result{}, ErrNoRifle
		}
		if gs.Ammo <= 0 {
			return organ.Result{}, ErrNoAmmo
		}
		gs.Ammo--
		gs.Pet.Hunger += 50
		gs.Pet.Energy -= 20

	case Wrestle:
		gs.Pet.Happiness += 30
		gs.Pet.Energy -= 25
		gs.Pet.Hunger -= 15

	case Patrol:
		gs.Pet.Energy -= 15

	case Work:
		gs.Wallet += 10

	case Idle:
		// nothing

	default:
		return organ.Result{}, ErrUnknownAction
	}

	gs.LastAction = action
	gs.ConsecutiveOf = consecutive

	pet := Pet{}
	pet.clamp(&gs.Pet)

	return organ.TextResult(action.String()), err
}

func AvailableActions(gs *GameState) []Action {
	actions := []Action{Feed, Rest, Play, Clean, Comfort, Work, Idle}

	if gs.Pet.Health < 40 {
		actions = append(actions, Medicine)
	}
	if gs.Stage >= Adolescent {
		actions = append(actions, Hunt)
	}
	if gs.Stage >= Adult {
		actions = append(actions, Patrol)
	}
	if gs.Stage >= Kraken {
		actions = append(actions, Wrestle)
	}
	return actions
}

func GameOrgans(gs *GameState) []organ.Func {
	actions := AvailableActions(gs)
	fns := make([]organ.Func, len(actions))
	for i, a := range actions {
		af := NewActionFunction(a, gs); fns[i] = organ.Func{Name: af.Name(), Description: af.Description(), Schema: af.InputSchema(), Mode: organ.WriteAction, Source: organ.Environment, Execute: af.Execute}
	}
	return fns
}

func GameCapabilitySet(gs *GameState) *organ.FuncSet {
	cs := organ.NewFuncSet(); for _, c := range GameOrgans(gs) { cs.Register(c) }; return cs
}

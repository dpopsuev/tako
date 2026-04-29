package minitako

import (
	"encoding/json"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/fab"
)

func ArcadeStation() fab.Station {
	return fab.Station{
		Name:        "arcade",
		Instruments: []string{"feed", "rest", "play", "clean", "medicine", "comfort", "hunt", "wrestle", "patrol", "work", "shop", "browse", "idle"},
		Intake:      true,
	}
}

func ArcadeAssembly() fab.Assembly {
	station := ArcadeStation()
	return fab.Assembly{
		Name:     "minitako",
		Stations: map[string]fab.Station{station.Name: station},
		Contracts: []fab.Contract{{
			From:      station.Name,
			To:        station.Name,
			Evaluator: NewDayContract(),
		}},
	}
}

func SeedArtifact() artifact.Envelope {
	gs := NewGameState()
	gs.ActionTicker = 14
	return PackState(&gs)
}

func NightTransition(env artifact.Envelope) (artifact.Envelope, error) {
	gs, err := UnpackState(env)
	if err != nil {
		return artifact.Envelope{}, err
	}

	pet := Pet{}

	for h := 21; h < 24; h++ {
		pet.NightDecay(&gs)
		pet.Cascade(&gs)
		if dead, cause := pet.CheckDeath(&gs); dead {
			gs.Alive = false
			gs.DeathCause = cause
			return PackState(&gs), nil
		}
		pet.Grow(&gs)
	}
	for h := 0; h < 6; h++ {
		pet.NightDecay(&gs)
		pet.Cascade(&gs)
		if dead, cause := pet.CheckDeath(&gs); dead {
			gs.Alive = false
			gs.DeathCause = cause
			return PackState(&gs), nil
		}
		pet.Grow(&gs)
	}

	gs.ActionTicker = 14
	gs.Hour = 6
	gs.Day++

	return PackState(&gs), nil
}

type NightContractEvaluator struct{}

func (NightContractEvaluator) Evaluate(_ fab.Contract, env artifact.Envelope) (bool, error) {
	gs, err := UnpackState(env)
	if err != nil {
		return false, err
	}
	if !gs.Alive {
		return true, nil
	}
	return gs.ActionTicker <= 0, nil
}

func RunGame(player Player, inspector GameInspector) RunResult {
	gs := NewGameState()
	gs.ActionTicker = 14
	pet := Pet{}

	var totalScore float64
	var ticks int

	for gs.Alive && gs.Day <= 7 {
		if gs.ActionTicker <= 0 {
			env := PackState(&gs)
			env, _ = NightTransition(env)
			gs, _ = UnpackState(env)
			if !gs.Alive {
				break
			}
			continue
		}

		action := player.Act(gs)
		score := inspector.Score(gs, action)
		totalScore += score

		ApplyAction(&gs, action)
		pet.Decay(&gs)
		pet.Cascade(&gs)

		if dead, cause := pet.CheckDeath(&gs); dead {
			gs.Alive = false
			gs.DeathCause = cause
			break
		}

		pet.Grow(&gs)
		gs.ActionTicker--
		gs.Hour++
		ticks++
	}

	oae := 0.0
	if ticks > 0 {
		oae = totalScore / float64(ticks)
	}
	return RunResult{
		PeakAge:       gs.Age,
		PeakStage:     gs.Stage,
		DaysSurvived:  gs.Day,
		TicksSurvived: ticks,
		OAE:           oae,
		DeathCause:    gs.DeathCause,
	}
}

func MarshalState(gs *GameState) []byte {
	data, _ := json.Marshal(gs)
	return data
}

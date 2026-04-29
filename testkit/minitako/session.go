package minitako

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/sleep"
)

type SessionConfig struct {
	Shelf     depo.Shelf
	Drain     sleep.Drain
	Mesh      memory.Mesh
	Monolog   discourse.Monolog
	Player    Player
	Inspector GameInspector
	MaxDays   int
}

func RunSession(ctx context.Context, cfg SessionConfig) RunResult {
	shelf := cfg.Shelf
	watch := shelf.Watch()

	seed := SeedArtifact()
	shelf.Push(seed)

	var runs []RunResult
	pet := Pet{}

	for {
		select {
		case <-ctx.Done():
			return aggregate(runs)
		case env := <-watch:
			gs, err := UnpackState(env)
			if err != nil {
				return aggregate(runs)
			}

			if !gs.Alive {
				return aggregate(runs)
			}

			if gs.Day > cfg.MaxDays {
				gs.DeathCause = "survived"
				return aggregate(runs)
			}

			result := workDay(ctx, &gs, cfg.Player, cfg.Inspector, &pet)

			if cfg.Monolog != nil {
				cfg.Monolog.Write(discourse.Letter{
					From:    "minitako",
					Subject: fmt.Sprintf("day-%d-complete", gs.Day),
					Body:    fmt.Sprintf("stage=%s ticks=%d oae=%.2f cause=%s", result.PeakStage, result.TicksSurvived, result.OAE, result.DeathCause),
					CreatedAt: time.Now(),
				})
			}

			runs = append(runs, result)

			if cfg.Drain != nil && cfg.Mesh != nil {
				cfg.Drain.Sweep(cfg.Mesh)
			}

			if !gs.Alive {
				return aggregate(runs)
			}

			nightEnv, err := NightTransition(PackState(&gs))
			if err != nil {
				return aggregate(runs)
			}

			nightGs, _ := UnpackState(nightEnv)
			if !nightGs.Alive {
				return aggregate(runs)
			}

			shelf.Push(nightEnv)
		}
	}
}

func workDay(_ context.Context, gs *GameState, player Player, inspector GameInspector, pet *Pet) RunResult {
	var totalScore float64
	var ticks int

	for gs.Alive && gs.ActionTicker > 0 {
		action := player.Act(*gs)
		score := inspector.Score(*gs, action)
		totalScore += score

		ApplyAction(gs, action)
		pet.Decay(gs)
		pet.Cascade(gs)

		if dead, cause := pet.CheckDeath(gs); dead {
			gs.Alive = false
			gs.DeathCause = cause
			break
		}

		pet.Grow(gs)
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

func aggregate(runs []RunResult) RunResult {
	if len(runs) == 0 {
		return RunResult{}
	}
	last := runs[len(runs)-1]
	totalTicks := 0
	totalOAE := 0.0
	for _, r := range runs {
		totalTicks += r.TicksSurvived
		totalOAE += r.OAE
	}
	last.TicksSurvived = totalTicks
	if len(runs) > 0 {
		last.OAE = totalOAE / float64(len(runs))
	}
	return last
}

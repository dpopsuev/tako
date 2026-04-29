package minitako

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/sleep"
)

func TestRunSession_EventDriven(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dp := depo.NewStubDepo("minitako")
	shelf := dp.Shelf("arcade")
	monolog := &discourse.StubMonolog{}
	mesh := memory.NewStubMesh()
	drain := sleep.NewDoltDrain(monolog)

	result := RunSession(ctx, SessionConfig{
		Shelf:     shelf,
		Drain:     drain,
		Mesh:      mesh,
		Monolog:   monolog,
		Player:    RandomPlayer{},
		Inspector: StubInspector{},
		MaxDays:   7,
	})

	t.Logf("Session: %d ticks, %d days, peak=%s, died=%s, OAE=%.2f",
		result.TicksSurvived, result.DaysSurvived, result.PeakStage, result.DeathCause, result.OAE)

	if result.TicksSurvived == 0 {
		t.Error("should survive at least 1 tick")
	}
	if result.DaysSurvived < 1 {
		t.Error("should survive at least 1 day")
	}
}

func TestRunSession_DrainWritesToMesh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dp := depo.NewStubDepo("minitako")
	shelf := dp.Shelf("arcade")
	monolog := &discourse.StubMonolog{}
	mesh := memory.NewStubMesh()
	drain := sleep.NewDoltDrain(monolog)

	RunSession(ctx, SessionConfig{
		Shelf:     shelf,
		Drain:     drain,
		Mesh:      mesh,
		Monolog:   monolog,
		Player:    RandomPlayer{},
		Inspector: StubInspector{},
		MaxDays:   7,
	})

	letters := monolog.Letters()
	nodes := mesh.Nodes()

	t.Logf("Monolog letters: %d, Mesh nodes: %d", len(letters), len(nodes))

	if len(letters) == 0 {
		t.Error("Monolog should have letters after session")
	}
	if len(nodes) == 0 {
		t.Error("Mesh should have nodes after drain")
	}
}

func TestRunSession_WatchDrivesWake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dp := depo.NewStubDepo("minitako")
	shelf := dp.Shelf("arcade")

	result := RunSession(ctx, SessionConfig{
		Shelf:     shelf,
		Player:    RandomPlayer{},
		Inspector: StubInspector{},
		MaxDays:   3,
	})

	if result.DaysSurvived < 1 {
		t.Error("Watch should drive at least 1 day cycle")
	}
	t.Logf("Watch-driven session: %d days, cause=%s", result.DaysSurvived, result.DeathCause)
}

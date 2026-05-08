package framework

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/agent"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/fab"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/observe"
	"github.com/dpopsuev/tako/render"
	"github.com/dpopsuev/tako/service/andon"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/kanban"
	"github.com/dpopsuev/tako/store"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWalkingSkeleton(t *testing.T) {
	assembly := fab.StubAssembly()
	kb := kanban.NewStubBoard(assembly)
	an := &andon.StubSignal{}
	pool := &ergograph.StubLedger{}
	inspector := ergograph.StubInspector{}
	canvas := render.NewStubCanvas()
	mesh := memory.NewStubMesh()
	dp := depo.NewStubDepo("test")
	lobby := agent.StubLobby{}
	runner := &agent.StubRunner{}

	caps := organ.NewFuncSet()
	caps.Register(organ.Func{
		Name:        "echo",
		Description: "echo input",
		Source:      organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			return organ.TextResult(string(input)), nil
		},
	})

	fc := NewFabCollective(FabCollectiveConfig{
		Assembly:     assembly,
		Kanban:       kb,
		Andon:        an,
		Pool:         pool,
		Inspector:    inspector,
		Canvas:       canvas,
		Depo:         dp,
		Lobby:        lobby,
		Mesh:         mesh,
		Runner:       runner,
		Capabilities: caps,
	})

	ctx := context.Background()
	if err := fc.Run(ctx); err != nil {
		t.Fatalf("FabCollective.Run() failed: %v", err)
	}

	// 1. Agent was created with Corpus
	agents := fc.Agents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Persona != agent.Worker {
		t.Errorf("expected Worker persona, got %s", agents[0].Persona)
	}

	// 2. Corpus assembled
	_ = agents[0].Corpus

	// 3. Reactivity completed (runner executed)
	if !runner.Executed {
		t.Error("agent runner was not executed")
	}

	// 5. Kanban station was claimed
	stations := kb.Stations()
	var intakeClaimed bool
	for _, s := range stations {
		if s.Name == "intake" && !s.Claimable {
			intakeClaimed = true
		}
	}
	if !intakeClaimed {
		t.Error("intake station was not claimed on kanban board")
	}

	// 6. Ergograph has a record
	if pool.Len() != 1 {
		t.Errorf("expected 1 ergograph record, got %d", pool.Len())
	}
	if err := pool.VerifyChain(); err != nil {
		t.Errorf("ergograph chain verification failed: %v", err)
	}

	// 7. Inspector scores perfect OAE
	oae, err := inspector.Score(pool)
	if err != nil {
		t.Fatalf("inspector.Score failed: %v", err)
	}
	if oae.Score() != 1.0 {
		t.Errorf("expected OAE 1.0, got %f", oae.Score())
	}

	// 8. Memory mesh has a knowledge node
	meshNodes := mesh.Nodes()
	if len(meshNodes) != 1 {
		t.Errorf("expected 1 memory node, got %d", len(meshNodes))
	}

	// 9. Monolog has a letter
	letters := fc.Monolog().Letters()
	if len(letters) != 1 {
		t.Errorf("expected 1 monolog letter, got %d", len(letters))
	}

	// 10. Canvas received panel posts (station + OAE)
	if canvas.PanelCount() < 2 {
		t.Errorf("expected at least 2 canvas panels (station + OAE), got %d", canvas.PanelCount())
	}

	// 11. Andon stayed green
	if an.Status() != andon.Green {
		t.Errorf("expected andon Green, got %s", an.Status())
	}

	// 12. Depo has envelope on terminus shelf
	assertDepoShelf(t, dp, "terminus", 1)
}

func TestWalkingSkeletonDolt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "takodb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	tracer := tp.Tracer("tako-test")

	assembly := fab.StubAssembly()
	kb := kanban.NewStubBoard(assembly)
	an := &andon.StubSignal{}

	doltPool := ergograph.NewDoltLedger(db.DB)
	pool := observe.NewLedger(doltPool, tracer, "main")

	inspector := ergograph.StubInspector{}
	canvas := render.NewStubCanvas()
	mesh := memory.NewStubMesh()
	dp := depo.NewDoltDepo(db.DB, "test")
	lobby := agent.StubLobby{}
	runner := &agent.StubRunner{}

	caps := organ.NewFuncSet()
	caps.Register(organ.Func{
		Name:        "echo",
		Description: "echo input",
		Source:      organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			return organ.TextResult(string(input)), nil
		},
	})

	fc := NewFabCollective(FabCollectiveConfig{
		Assembly:     assembly,
		Kanban:       kb,
		Andon:        an,
		Pool:         pool,
		Inspector:    inspector,
		Canvas:       canvas,
		Depo:         dp,
		Lobby:        lobby,
		Mesh:         mesh,
		Runner:       runner,
		Capabilities: caps,
	})

	ctx := context.Background()
	if err := fc.Run(ctx); err != nil {
		t.Fatalf("FabCollective.Run() with Dolt failed: %v", err)
	}

	// Ergograph records persisted in Dolt
	if doltPool.Len() < 1 {
		t.Errorf("expected ergograph records in Dolt, got %d", doltPool.Len())
	}
	if err := doltPool.VerifyChain(); err != nil {
		t.Errorf("Dolt ergograph chain broken: %v", err)
	}

	// Depo has envelope on terminus shelf (Dolt-backed)
	assertDepoShelf(t, dp, "terminus", 1)

	// OTel spans were emitted
	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Error("expected OTel spans from observe/ decorators")
	}

	// Inspector still scores from Dolt pool
	oae, err := inspector.Score(doltPool)
	if err != nil {
		t.Fatalf("inspector.Score: %v", err)
	}
	if oae.Score() != 1.0 {
		t.Errorf("expected OAE 1.0, got %f", oae.Score())
	}
}

func assertDepoShelf(t *testing.T, dp depo.Depo, shelfName string, expectedCount int) {
	t.Helper()
	shelf := dp.Shelf(shelfName)
	items := shelf.Peek()
	if len(items) != expectedCount {
		t.Errorf("expected %d envelopes on %s shelf, got %d", expectedCount, shelfName, len(items))
	}
}

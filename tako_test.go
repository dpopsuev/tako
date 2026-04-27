package framework

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/agent"
	"github.com/dpopsuev/origami/ergograph"
	"github.com/dpopsuev/origami/fab"
	"github.com/dpopsuev/origami/memory"
	"github.com/dpopsuev/origami/render"
	"github.com/dpopsuev/origami/service/andon"
	"github.com/dpopsuev/origami/service/depo"
	"github.com/dpopsuev/origami/service/kanban"
	"github.com/dpopsuev/origami/workstation"
)

func TestWalkingSkeleton(t *testing.T) {
	assembly := fab.StubAssembly()
	kb := kanban.NewStubBoard(assembly)
	an := &andon.StubSignal{}
	pool := &ergograph.StubPool{}
	inspector := ergograph.StubInspector{}
	canvas := &render.StubCanvas{}
	mesh := memory.NewStubMesh()
	ws := workstation.NewStubWorkstation()
	dp := depo.NewStubDepo("test")
	lobby := agent.StubLobby{}
	runner := &agent.StubRunner{}

	fc := NewFabCollective(FabCollectiveConfig{
		Assembly:    assembly,
		Kanban:      kb,
		Andon:       an,
		Pool:        pool,
		Inspector:   inspector,
		Canvas:      canvas,
		Depo:        dp,
		Lobby:       lobby,
		Mesh:        mesh,
		Workstation: ws,
		Runner:      runner,
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

	// 2. Corpus has Organs
	organs := agents[0].Corpus.Organs()
	if len(organs) < 5 {
		t.Errorf("expected at least 5 organs, got %d", len(organs))
	}

	// 3. Reactivity completed (runner executed)
	if !runner.Executed {
		t.Error("agent runner was not executed")
	}

	// 4. Workstation provisioned with shell
	shell := ws.Shell()
	if shell == nil {
		t.Fatal("workstation shell is nil after provision")
	}
	names := shell.Names()
	if len(names) == 0 {
		t.Fatal("shell has no instruments")
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

	// 9. Monologue has a letter
	letters := fc.Monologue().Letters()
	if len(letters) != 1 {
		t.Errorf("expected 1 monologue letter, got %d", len(letters))
	}

	// 10. Canvas received damage notification
	if canvas.DamageCount() != 1 {
		t.Errorf("expected 1 canvas damage, got %d", canvas.DamageCount())
	}

	// 11. Andon stayed green
	if an.Status() != andon.Green {
		t.Errorf("expected andon Green, got %s", an.Status())
	}

	// 12. Depo has envelope on terminus shelf
	assertDepoShelf(t, dp, "terminus", 1)
}

func assertDepoShelf(t *testing.T, dp *depo.StubDepo, shelfName string, expectedCount int) {
	t.Helper()
	shelf := dp.Shelf(shelfName)
	items := shelf.Peek()
	if len(items) != expectedCount {
		t.Errorf("expected %d envelopes on %s shelf, got %d", expectedCount, shelfName, len(items))
	}
}

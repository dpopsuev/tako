package cerebrum

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/instrument"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/service/sleep"
	"github.com/dpopsuev/tako/store"
)

func TestThink_FullVerticalSlice(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdb")
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	completer := &instrument.StubCompleter{Response: []byte("done")}
	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)

	m, err := cb.Think(context.Background(), []byte("investigate PTP failure"))
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	if !m.Sealed() {
		t.Fatal("Molecule should be sealed")
	}

	if m.Mass(reactivity.IntentAtom) == 0 {
		t.Error("missing Intent atoms")
	}
	if m.Mass(reactivity.RetrospectionAtom) == 0 {
		t.Error("missing Retrospection atoms (Wish)")
	}

	monolog := &discourse.StubMonolog{}
	monolog.Write(discourse.Letter{
		From:    "cerebrum",
		Subject: "think-complete",
		Body:    string(m.Atoms(reactivity.RetrospectionAtom)[0].Content),
	})

	mesh := memory.NewDoltMesh(db.DB)
	drain := sleep.NewDoltDrain(monolog)

	if err := drain.Sweep(mesh); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	nodes := mesh.Nodes()
	if len(nodes) == 0 {
		t.Fatal("DoltMesh should have knowledge after drain")
	}

	t.Logf("Vertical slice complete: %d atoms in Molecule, %d nodes in Mesh", m.TotalMass(), len(nodes))
}

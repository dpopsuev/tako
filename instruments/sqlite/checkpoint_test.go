package sqlite

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func newTestCP(t *testing.T) *SQLiteCheckpointer {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cp, err := NewCheckpointer(dbPath)
	if err != nil {
		t.Fatalf("NewCheckpointer: %v", err)
	}
	t.Cleanup(func() { cp.Close() })
	return cp
}

func TestSQLiteCheckpointer_InterfaceCompliance(t *testing.T) {
	var _ circuit.Checkpointer = (*SQLiteCheckpointer)(nil)
}

func TestSQLiteCheckpointer_SaveAndLoad(t *testing.T) {
	cp := newTestCP(t)

	state := circuit.NewWalkerState("walk-1")
	state.CurrentNode = "investigate"
	state.LoopCounts["investigate"] = 2
	state.Context["key"] = "value"

	if err := cp.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := cp.Load("walk-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.ID != "walk-1" {
		t.Errorf("ID = %q, want walk-1", loaded.ID)
	}
	if loaded.CurrentNode != "investigate" {
		t.Errorf("CurrentNode = %q, want investigate", loaded.CurrentNode)
	}
	if loaded.LoopCounts["investigate"] != 2 {
		t.Errorf("LoopCounts = %v", loaded.LoopCounts)
	}
}

func TestSQLiteCheckpointer_LoadNotFound(t *testing.T) {
	cp := newTestCP(t)
	state, err := cp.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state != nil {
		t.Error("expected nil for missing checkpoint")
	}
}

func TestSQLiteCheckpointer_Remove(t *testing.T) {
	cp := newTestCP(t)
	cp.Save(circuit.NewWalkerState("walk-rm"))

	if err := cp.Remove("walk-rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	loaded, _ := cp.Load("walk-rm")
	if loaded != nil {
		t.Error("checkpoint should be removed")
	}
}

func TestSQLiteCheckpointer_List(t *testing.T) {
	cp := newTestCP(t)
	cp.Save(circuit.NewWalkerState("a"))
	cp.Save(circuit.NewWalkerState("b"))
	cp.Save(circuit.NewWalkerState("c"))

	ids, err := cp.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3, got %d", len(ids))
	}
}

func TestSQLiteCheckpointer_Overwrite(t *testing.T) {
	cp := newTestCP(t)

	state := circuit.NewWalkerState("ow")
	state.CurrentNode = "a"
	cp.Save(state)

	state.CurrentNode = "b"
	cp.Save(state)

	loaded, _ := cp.Load("ow")
	if loaded.CurrentNode != "b" {
		t.Errorf("CurrentNode = %q, want b (overwritten)", loaded.CurrentNode)
	}
}

func TestSQLiteCheckpointer_ConcurrentAccess(t *testing.T) {
	cp := newTestCP(t)
	const workers = 10

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("walk-%d", n)
			state := circuit.NewWalkerState(id)
			state.CurrentNode = "test"
			if err := cp.Save(state); err != nil {
				t.Errorf("Save %s: %v", id, err)
			}
			loaded, err := cp.Load(id)
			if err != nil {
				t.Errorf("Load %s: %v", id, err)
			}
			if loaded == nil || loaded.ID != id {
				t.Errorf("Load %s: unexpected result", id)
			}
		}(i)
	}
	wg.Wait()

	ids, _ := cp.List()
	if len(ids) != workers {
		t.Errorf("List: expected %d, got %d", workers, len(ids))
	}
}

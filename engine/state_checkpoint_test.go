package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

func TestJSONCheckpointer_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cp, err := NewJSONCheckpointer(dir)
	if err != nil {
		t.Fatalf("NewJSONCheckpointer: %v", err)
	}

	state := circuit.NewWalkerState("walk-1")
	state.Status = "running"
	state.LoopCounts["investigate"] = 2
	state.Context["key"] = "value"
	state.RecordStep("triage", "needs-investigation", "E2", "2026-01-01T00:00:00Z")
	state.CurrentNode = "investigate"

	if err := cp.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, "walk-1.checkpoint.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("checkpoint file not created: %v", err)
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
		t.Errorf("LoopCounts[investigate] = %d, want 2", loaded.LoopCounts["investigate"])
	}
	if loaded.Context["key"] != "value" {
		t.Errorf("Context[key] = %v, want value", loaded.Context["key"])
	}
	if len(loaded.History) != 1 {
		t.Errorf("len(History) = %d, want 1", len(loaded.History))
	}
	if loaded.History[0].Node != "triage" {
		t.Errorf("History[0].Node = %q, want triage", loaded.History[0].Node)
	}
}

func TestJSONCheckpointer_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	cp, _ := NewJSONCheckpointer(dir)

	state, err := cp.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load should not error for missing checkpoint: %v", err)
	}
	if state != nil {
		t.Error("Load should return nil for missing checkpoint")
	}
}

func TestJSONCheckpointer_Remove(t *testing.T) {
	dir := t.TempDir()
	cp, _ := NewJSONCheckpointer(dir)

	state := circuit.NewWalkerState("walk-rm")
	cp.Save(state)

	if err := cp.Remove("walk-rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	loaded, _ := cp.Load("walk-rm")
	if loaded != nil {
		t.Error("checkpoint should be removed")
	}
}

func TestJSONCheckpointer_Remove_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	cp, _ := NewJSONCheckpointer(dir)

	if err := cp.Remove("nonexistent"); err != nil {
		t.Fatalf("Remove nonexistent should not error: %v", err)
	}
}

func TestJSONCheckpointer_Load_InitializesMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.checkpoint.json")
	os.WriteFile(path, []byte(`{"id":"minimal","status":"done"}`), 0o644)

	cp, _ := NewJSONCheckpointer(dir)
	state, err := cp.Load("minimal")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.LoopCounts == nil {
		t.Error("LoopCounts should be initialized")
	}
	if state.Context == nil {
		t.Error("Context should be initialized")
	}
	if state.Outputs == nil {
		t.Error("Outputs should be initialized")
	}
}

func TestJSONCheckpointer_Overwrite(t *testing.T) {
	dir := t.TempDir()
	cp, _ := NewJSONCheckpointer(dir)

	state := circuit.NewWalkerState("walk-ow")
	state.CurrentNode = "triage"
	cp.Save(state)

	state.CurrentNode = "investigate"
	state.LoopCounts["investigate"] = 1
	cp.Save(state)

	loaded, _ := cp.Load("walk-ow")
	if loaded.CurrentNode != "investigate" {
		t.Errorf("CurrentNode = %q, want investigate (overwritten)", loaded.CurrentNode)
	}
}

func TestJSONCheckpointer_List(t *testing.T) {
	dir := t.TempDir()
	cp, _ := NewJSONCheckpointer(dir)

	ids, err := cp.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs, got %d", len(ids))
	}

	cp.Save(circuit.NewWalkerState("walk-a"))
	cp.Save(circuit.NewWalkerState("walk-b"))
	cp.Save(circuit.NewWalkerState("walk-c"))

	ids, err = cp.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	for _, want := range []string{"walk-a", "walk-b", "walk-c"} {
		if !found[want] {
			t.Errorf("missing ID %q in List()", want)
		}
	}
}

func TestJSONCheckpointer_InterfaceCompliance(t *testing.T) {
	var _ circuit.Checkpointer = (*JSONCheckpointer)(nil)
}

package ouroboros

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func TestFileRunStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileRunStore(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	report := RunReport{
		RunID:     "test-run-001",
		StartTime: time.Date(2026, 2, 21, 15, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 21, 15, 5, 0, 0, time.UTC),
		Config:    DefaultConfig(),
		Results: []DiscoveryResult{
			{
				Iteration: 0,
				Model:     circuit.ModelIdentity{ModelName: "gpt-4o", Provider: "OpenAI"},
				Probe: ProbeResult{
					ProbeID: "refactor-v1",
					Score:   ProbeScore{Renames: 3, TotalScore: 0.65},
				},
			},
		},
		UniqueModels: []circuit.ModelIdentity{
			{ModelName: "gpt-4o", Provider: "OpenAI"},
		},
		TermReason: "repeat",
	}

	if err := store.SaveRun(report); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadRun("test-run-001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.RunID != "test-run-001" {
		t.Errorf("run_id: got %q", loaded.RunID)
	}
	if len(loaded.Results) != 1 {
		t.Fatalf("results: got %d, want 1", len(loaded.Results))
	}
	if loaded.Results[0].Model.ModelName != "gpt-4o" {
		t.Errorf("model: got %q", loaded.Results[0].Model.ModelName)
	}
	if loaded.TermReason != "repeat" {
		t.Errorf("term_reason: got %q", loaded.TermReason)
	}
}

func TestFileRunStore_AppendOnly(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileRunStore(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	report := RunReport{RunID: "run-dup"}
	if err := store.SaveRun(report); err != nil {
		t.Fatalf("first save: %v", err)
	}

	err = store.SaveRun(report)
	if err == nil {
		t.Fatal("expected error on duplicate save (append-only violation)")
	}
}

func TestFileRunStore_ListRuns(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileRunStore(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	for _, id := range []string{"run-003", "run-001", "run-002"} {
		if err := store.SaveRun(RunReport{RunID: id}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	ids, err := store.ListRuns()
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(ids))
	}
	if ids[0] != "run-001" || ids[1] != "run-002" || ids[2] != "run-003" {
		t.Errorf("expected sorted order, got %v", ids)
	}
}

func TestFileRunStore_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileRunStore(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	_, err = store.LoadRun("nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent run")
	}
}

func TestFileRunStore_IndependentRuns(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileRunStore(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	run1 := RunReport{
		RunID:      "run-alpha",
		TermReason: "repeat",
		UniqueModels: []circuit.ModelIdentity{
			{ModelName: "model-a", Provider: "ProvA"},
		},
	}
	run2 := RunReport{
		RunID:      "run-beta",
		TermReason: "max_iterations",
		UniqueModels: []circuit.ModelIdentity{
			{ModelName: "model-b", Provider: "ProvB"},
			{ModelName: "model-c", Provider: "ProvC"},
		},
	}

	if err := store.SaveRun(run1); err != nil {
		t.Fatalf("save run1: %v", err)
	}
	if err := store.SaveRun(run2); err != nil {
		t.Fatalf("save run2: %v", err)
	}

	loaded1, err := store.LoadRun("run-alpha")
	if err != nil {
		t.Fatalf("load run1: %v", err)
	}
	loaded2, err := store.LoadRun("run-beta")
	if err != nil {
		t.Fatalf("load run2: %v", err)
	}

	if len(loaded1.UniqueModels) != 1 {
		t.Errorf("run1 models: got %d, want 1", len(loaded1.UniqueModels))
	}
	if len(loaded2.UniqueModels) != 2 {
		t.Errorf("run2 models: got %d, want 2", len(loaded2.UniqueModels))
	}
	if loaded1.TermReason != "repeat" {
		t.Errorf("run1 reason: got %q", loaded1.TermReason)
	}
	if loaded2.TermReason != "max_iterations" {
		t.Errorf("run2 reason: got %q", loaded2.TermReason)
	}
}

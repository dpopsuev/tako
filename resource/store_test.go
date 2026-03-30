package resource

import (
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func scenarioYAML(name string) []byte {
	return []byte("kind: scenario\nversion: v1\nmetadata:\n  name: " + name + "\ncases: []\n")
}

func scorecardYAML(name string) []byte {
	return []byte("kind: scorecard\nversion: v1\nmetadata:\n  name: " + name + "\nmetrics:\n  - id: M1\n    name: accuracy\n    scorer: exact_match\n    threshold: 0.7\n")
}

func TestLiveStore_CRUD(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	// Create
	res, err := store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Kind != circuit.KindScenario {
		t.Errorf("Kind = %q", res.Kind)
	}
	if res.Version != "1" {
		t.Errorf("Version = %q, want 1", res.Version)
	}

	// Get
	res, err = store.Get(circuit.KindScenario, "ptp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Metadata.Name != "ptp" {
		t.Errorf("Name = %q", res.Metadata.Name)
	}

	// Update
	res, err = store.Update(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Version != "2" {
		t.Errorf("Version = %q, want 2", res.Version)
	}

	// List
	all, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("List = %d, want 1", len(all))
	}

	// List filtered
	all, err = store.List(circuit.KindScorecard)
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List scorecard = %d, want 0", len(all))
	}
}

func TestLiveStore_Rollback(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	store.Update(circuit.KindScenario, "ptp", scenarioYAML("ptp-v2"))
	store.Update(circuit.KindScenario, "ptp", scenarioYAML("ptp-v3"))

	// Rollback to v1
	res, err := store.Rollback(circuit.KindScenario, "ptp", 1)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if res.Version != "4" {
		t.Errorf("Version = %q, want 4", res.Version)
	}
}

func TestLiveStore_History(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	store.Update(circuit.KindScenario, "ptp", scenarioYAML("ptp-v2"))

	history, err := store.History(circuit.KindScenario, "ptp")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("History = %d, want 2", len(history))
	}
}

func TestLiveStore_CreateDuplicate(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	_, err := store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	if !errors.Is(err, ErrStoreAlreadyExists) {
		t.Errorf("expected ErrStoreAlreadyExists, got %v", err)
	}
}

func TestLiveStore_GetNotFound(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	_, err := store.Get(circuit.KindScenario, "nonexistent")
	if !errors.Is(err, ErrStoreNotFound) {
		t.Errorf("expected ErrStoreNotFound, got %v", err)
	}
}

func TestLiveStore_MultiKind(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	store.Create(circuit.KindScenario, "ptp", scenarioYAML("ptp"))
	store.Create(circuit.KindScorecard, "rca", scorecardYAML("rca"))

	all, _ := store.List("")
	if len(all) != 2 {
		t.Errorf("List all = %d, want 2", len(all))
	}

	scenarios, _ := store.List(circuit.KindScenario)
	if len(scenarios) != 1 {
		t.Errorf("List scenario = %d, want 1", len(scenarios))
	}

	scorecards, _ := store.List(circuit.KindScorecard)
	if len(scorecards) != 1 {
		t.Errorf("List scorecard = %d, want 1", len(scorecards))
	}
}

func TestLiveStore_SeedFrom(t *testing.T) {
	reg := DefaultRegistry()
	store := NewLiveStore(reg)

	// Build a ResourceIndex manually.
	idx := &ResourceIndex{
		byKind: map[circuit.Kind][]*Resource{
			circuit.KindScenario: {
				{Kind: circuit.KindScenario, Version: "v1", Metadata: Metadata{Name: "seeded"}, Raw: scenarioYAML("seeded")},
			},
		},
		byKey: map[string]*Resource{},
		all: []*Resource{
			{Kind: circuit.KindScenario, Version: "v1", Metadata: Metadata{Name: "seeded"}, Raw: scenarioYAML("seeded")},
		},
	}

	store.SeedFrom(idx)

	res, err := store.Get(circuit.KindScenario, "seeded")
	if err != nil {
		t.Fatalf("Get seeded: %v", err)
	}
	if res.Metadata.Name != "seeded" {
		t.Errorf("Name = %q", res.Metadata.Name)
	}

	// Should be updatable after seeding.
	res, err = store.Update(circuit.KindScenario, "seeded", scenarioYAML("seeded"))
	if err != nil {
		t.Fatalf("Update seeded: %v", err)
	}
	if res.Version != "2" {
		t.Errorf("Version = %q, want 2", res.Version)
	}
}

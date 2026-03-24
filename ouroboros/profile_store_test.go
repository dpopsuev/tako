package ouroboros

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/agentport"
)

func makeTestProfile(modelName string, ts time.Time) ModelProfile {
	return ModelProfile{
		Model:          circuit.ModelIdentity{ModelName: modelName, Provider: "test"},
		BatteryVersion: "ouroboros-v1",
		Timestamp:      ts,
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.8,
			DimPersistence:          0.3,
			DimConvergenceThreshold: 0.7,
			DimShortcutAffinity:     0.9,
			DimEvidenceDepth:        0.4,
			DimFailureMode:          0.5,
		},
		ElementMatch: agentport.ElementFire,
	}
}

func TestFileProfileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileProfileStore(dir)
	if err != nil {
		t.Fatalf("NewFileProfileStore: %v", err)
	}

	ts := time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)
	original := makeTestProfile("claude-sonnet-4", ts)

	if err := store.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("List: got %d, want 1", len(ids))
	}

	loaded, err := store.Load(ids[0])
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Model.ModelName != original.Model.ModelName {
		t.Errorf("ModelName = %q, want %q", loaded.Model.ModelName, original.Model.ModelName)
	}
	if loaded.BatteryVersion != original.BatteryVersion {
		t.Errorf("BatteryVersion = %q, want %q", loaded.BatteryVersion, original.BatteryVersion)
	}
	if len(loaded.Dimensions) != len(original.Dimensions) {
		t.Errorf("Dimensions count = %d, want %d", len(loaded.Dimensions), len(original.Dimensions))
	}
	for dim, want := range original.Dimensions {
		if got := loaded.Dimensions[dim]; got != want {
			t.Errorf("Dimension[%s] = %f, want %f", dim, got, want)
		}
	}
	if loaded.ElementMatch != original.ElementMatch {
		t.Errorf("ElementMatch = %q, want %q", loaded.ElementMatch, original.ElementMatch)
	}
}

func TestFileProfileStore_AppendOnly(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileProfileStore(dir)
	if err != nil {
		t.Fatalf("NewFileProfileStore: %v", err)
	}

	ts := time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)
	profile := makeTestProfile("gpt-4o", ts)

	if err := store.Save(profile); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	err = store.Save(profile)
	if err == nil {
		t.Fatal("second Save should fail (append-only)")
	}
}

func TestFileProfileStore_History(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileProfileStore(dir)
	if err != nil {
		t.Fatalf("NewFileProfileStore: %v", err)
	}

	ts1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	ts3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	if err := store.Save(makeTestProfile("claude-sonnet-4", ts2)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(makeTestProfile("claude-sonnet-4", ts1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(makeTestProfile("gpt-4o", ts3)); err != nil {
		t.Fatal(err)
	}

	history, err := store.History("claude-sonnet-4")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("History count = %d, want 2", len(history))
	}
	if !history[0].Timestamp.Before(history[1].Timestamp) {
		t.Error("History should be sorted oldest-first")
	}

	allHistory, err := store.History("gpt-4o")
	if err != nil {
		t.Fatalf("History(gpt-4o): %v", err)
	}
	if len(allHistory) != 1 {
		t.Fatalf("History(gpt-4o) count = %d, want 1", len(allHistory))
	}
}

func TestFileProfileStore_EmptyHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileProfileStore(dir)
	if err != nil {
		t.Fatalf("NewFileProfileStore: %v", err)
	}

	history, err := store.History("nonexistent")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("History count = %d, want 0", len(history))
	}
}

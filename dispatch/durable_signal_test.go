package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDurableSignalBus_EmitAndReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signals.jsonl")

	bus, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	bus.Emit("case_start", "worker-1", "C1", "recall", nil)
	bus.Emit("step_complete", "worker-1", "C1", "recall", map[string]string{"outcome": "hit"})
	bus.Emit("case_start", "worker-1", "C2", "recall", nil)

	if bus.Len() != 3 {
		t.Fatalf("expected 3 signals, got %d", bus.Len())
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Verify file exists and has content.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("signal file should not be empty")
	}

	// Create a new bus and replay.
	bus2, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatalf("create for replay: %v", err)
	}
	defer bus2.Close()

	count, err := bus2.Replay()
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if count != 3 {
		t.Errorf("replayed %d signals, want 3", count)
	}
	if bus2.Len() != 3 {
		t.Errorf("bus has %d signals after replay, want 3", bus2.Len())
	}

	signals := bus2.Since(0)
	if signals[0].Event != "case_start" {
		t.Errorf("first signal: got %s, want case_start", signals[0].Event)
	}
	if signals[1].Meta["outcome"] != "hit" {
		t.Error("second signal should have outcome=hit meta")
	}
}

func TestDurableSignalBus_ReplayMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.jsonl")

	bus, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer bus.Close()

	// Close first to release the write handle, then delete the file.
	bus.Close()
	os.Remove(path)

	// Replay on missing file should succeed with 0 count.
	bus2, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatalf("create for replay: %v", err)
	}
	defer bus2.Close()

	count, err := bus2.Replay()
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if count != 0 {
		t.Errorf("replay count: got %d, want 0", count)
	}
}

func TestDurableSignalBus_AppendAfterReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signals.jsonl")

	bus, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatal(err)
	}
	bus.Emit("start", "w1", "C1", "", nil)
	bus.Close()

	bus2, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatal(err)
	}
	defer bus2.Close()

	bus2.Replay()
	bus2.Emit("continue", "w1", "C1", "triage", nil)

	if bus2.Len() != 2 {
		t.Errorf("expected 2 signals, got %d", bus2.Len())
	}

	// Close and re-replay to verify both signals persisted.
	bus2.Close()

	bus3, err := NewDurableSignalBus(path)
	if err != nil {
		t.Fatal(err)
	}
	defer bus3.Close()
	count, _ := bus3.Replay()
	if count != 2 {
		t.Errorf("replayed %d, want 2", count)
	}
}

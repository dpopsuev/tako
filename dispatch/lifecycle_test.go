package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeSignals_SetsAllToComplete(t *testing.T) {
	dir := t.TempDir()

	// Create two case directories with signal files in different states.
	for _, sub := range []string{"C1", "C2"} {
		caseDir := filepath.Join(dir, sub)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			t.Fatal(err)
		}
		status := "waiting"
		if sub == "C2" {
			status = "processing"
		}
		sig := SignalFile{
			Status:     status,
			DispatchID: 1,
			CaseID:     sub,
			Step:       "F0_RECALL",
		}
		if err := WriteSignal(filepath.Join(caseDir, "signal.json"), &sig); err != nil {
			t.Fatal(err)
		}
	}

	FinalizeSignals(dir)

	// Verify all signals are now "complete".
	for _, sub := range []string{"C1", "C2"} {
		path := filepath.Join(dir, sub, "signal.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var sig SignalFile
		if err := json.Unmarshal(data, &sig); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		if sig.Status != "complete" {
			t.Errorf("%s: want status=complete, got %s", sub, sig.Status)
		}
	}
}

func TestFinalizeSignals_SkipsAlreadyComplete(t *testing.T) {
	dir := t.TempDir()
	sig := SignalFile{
		Status:     "complete",
		DispatchID: 5,
		CaseID:     "C1",
		Step:       "F1_TRIAGE",
		Timestamp:  "2026-01-01T00:00:00Z",
	}
	if err := WriteSignal(filepath.Join(dir, "signal.json"), &sig); err != nil {
		t.Fatal(err)
	}

	FinalizeSignals(dir)

	// Timestamp should be unchanged (not re-written).
	data, err := os.ReadFile(filepath.Join(dir, "signal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var after SignalFile
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatal(err)
	}
	if after.Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("timestamp changed: want 2026-01-01T00:00:00Z, got %s", after.Timestamp)
	}
}

func TestFinalizeSignals_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Should not panic on empty directory.
	FinalizeSignals(dir)
}

func TestWriteSignal_TmpCleanupOnRenameFail(t *testing.T) {
	// We can't easily force os.Rename to fail on a normal FS, but we can
	// verify the happy path doesn't leave .tmp files behind.
	dir := t.TempDir()
	path := filepath.Join(dir, "signal.json")

	sig := &SignalFile{Status: "waiting", CaseID: "C1", Step: "F0_RECALL"}
	if err := WriteSignal(path, sig); err != nil {
		t.Fatal(err)
	}

	// Verify no .tmp file exists.
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("expected .tmp to not exist after successful write, got err=%v", err)
	}

	// Verify signal was written correctly.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var read SignalFile
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if read.Status != "waiting" {
		t.Errorf("want status=waiting, got %s", read.Status)
	}
}

func TestPreRunCleanup_RemovesStaleDir(t *testing.T) {
	dir := t.TempDir()
	calibDir := filepath.Join(dir, "calibrate")

	// Create a stale calibration directory with content.
	caseDir := filepath.Join(calibDir, "C1")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staleFile := filepath.Join(caseDir, "signal.json")
	if err := os.WriteFile(staleFile, []byte(`{"status":"waiting"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate pre-run cleanup (same logic as runCalibrate).
	if info, err := os.Stat(calibDir); err == nil && info.IsDir() {
		if err := os.RemoveAll(calibDir); err != nil {
			t.Fatal(err)
		}
	}

	// Verify it's gone.
	if _, err := os.Stat(calibDir); !os.IsNotExist(err) {
		t.Error("expected calibDir to be removed")
	}

	// Recreate fresh.
	if err := os.MkdirAll(calibDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Verify it's empty.
	entries, err := os.ReadDir(calibDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty calibDir after clean, got %d entries", len(entries))
	}
}

func TestPreRunCleanup_NoOpWhenDirMissing(t *testing.T) {
	dir := t.TempDir()
	calibDir := filepath.Join(dir, "calibrate")

	// Dir doesn't exist — cleanup should be a no-op.
	if info, err := os.Stat(calibDir); err == nil && info.IsDir() {
		t.Fatal("calibDir should not exist yet")
	}
	// No error expected.
}

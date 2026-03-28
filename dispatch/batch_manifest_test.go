package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "batch-manifest.json")

	signals := []BatchSignalEntry{
		{CaseID: "C1", SignalPath: "/tmp/c1/signal.json", Status: "pending"},
		{CaseID: "C2", SignalPath: "/tmp/c2/signal.json", Status: "pending"},
		{CaseID: "C3", SignalPath: "/tmp/c3/signal.json", Status: "done"},
	}
	m := NewBatchManifest(7, "triage", "/tmp/briefing.md", signals)

	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.BatchID != 7 {
		t.Errorf("batch_id: got %d, want 7", got.BatchID)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %q, want pending", got.Status)
	}
	if got.Phase != "triage" {
		t.Errorf("phase: got %q, want triage", got.Phase)
	}
	if got.Total != 3 {
		t.Errorf("total: got %d, want 3", got.Total)
	}
	if got.BriefingPath != "/tmp/briefing.md" {
		t.Errorf("briefing_path: got %q", got.BriefingPath)
	}
	if len(got.Signals) != 3 {
		t.Fatalf("signals: got %d, want 3", len(got.Signals))
	}
	if got.Signals[2].Status != "done" {
		t.Errorf("signals[2].status: got %q, want done", got.Signals[2].Status)
	}
}

func TestWriteManifest_AtomicNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "batch-manifest.json")

	m := NewBatchManifest(1, "investigation", "", []BatchSignalEntry{
		{CaseID: "C1", SignalPath: "/x", Status: "pending"},
	})

	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("tmp file should not exist after write, got err: %v", err)
	}
}

func TestReadManifest_MissingFile(t *testing.T) {
	_, err := ReadManifest("/nonexistent/path/manifest.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteBudgetStatus_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget-status.json")

	if err := WriteBudgetStatus(path, 100000, 45000); err != nil {
		t.Fatalf("WriteBudgetStatus: %v", err)
	}

	got, err := ReadBudgetStatus(path)
	if err != nil {
		t.Fatalf("ReadBudgetStatus: %v", err)
	}

	if got.TotalBudget != 100000 {
		t.Errorf("total: got %d, want 100000", got.TotalBudget)
	}
	if got.Used != 45000 {
		t.Errorf("used: got %d, want 45000", got.Used)
	}
	if got.Remaining != 55000 {
		t.Errorf("remaining: got %d, want 55000", got.Remaining)
	}
	if got.PercentUsed != 45.0 {
		t.Errorf("percent_used: got %f, want 45.0", got.PercentUsed)
	}
}

func TestWriteBudgetStatus_ZeroBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget-status.json")

	if err := WriteBudgetStatus(path, 0, 0); err != nil {
		t.Fatalf("WriteBudgetStatus: %v", err)
	}

	got, err := ReadBudgetStatus(path)
	if err != nil {
		t.Fatalf("ReadBudgetStatus: %v", err)
	}
	if got.PercentUsed != 0.0 {
		t.Errorf("percent_used with zero budget: got %f, want 0.0", got.PercentUsed)
	}
}

func TestWriteBudgetStatus_OverBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget-status.json")

	if err := WriteBudgetStatus(path, 1000, 1500); err != nil {
		t.Fatalf("WriteBudgetStatus: %v", err)
	}

	got, err := ReadBudgetStatus(path)
	if err != nil {
		t.Fatalf("ReadBudgetStatus: %v", err)
	}
	if got.Remaining != -500 {
		t.Errorf("remaining: got %d, want -500", got.Remaining)
	}
	if got.PercentUsed != 150.0 {
		t.Errorf("percent_used: got %f, want 150.0", got.PercentUsed)
	}
}

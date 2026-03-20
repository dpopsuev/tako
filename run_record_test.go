package framework

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunRecord_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().Truncate(time.Second)
	rec := RunRecord{
		ID:          "s-123-1",
		Scenario:    "ptp",
		Backend:     "claude",
		Parallel:    4,
		StartedAt:   now,
		CompletedAt: now.Add(30 * time.Second),
		DurationMs:  30000,
		CaseCount:   18,
		ErrorCount:  2,
		TraceEvents: 42,
	}

	if err := SaveRunRecord(dir, rec); err != nil {
		t.Fatalf("SaveRunRecord: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(filepath.Join(dir, "run.json")); err != nil {
		t.Fatalf("run.json not created: %v", err)
	}

	got, err := LoadRunRecord(dir)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}

	if got.ID != rec.ID {
		t.Errorf("ID: got %q, want %q", got.ID, rec.ID)
	}
	if got.Scenario != rec.Scenario {
		t.Errorf("Scenario: got %q, want %q", got.Scenario, rec.Scenario)
	}
	if got.Backend != rec.Backend {
		t.Errorf("Backend: got %q, want %q", got.Backend, rec.Backend)
	}
	if got.Parallel != rec.Parallel {
		t.Errorf("Parallel: got %d, want %d", got.Parallel, rec.Parallel)
	}
	if !got.StartedAt.Equal(rec.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", got.StartedAt, rec.StartedAt)
	}
	if !got.CompletedAt.Equal(rec.CompletedAt) {
		t.Errorf("CompletedAt: got %v, want %v", got.CompletedAt, rec.CompletedAt)
	}
	if got.DurationMs != rec.DurationMs {
		t.Errorf("DurationMs: got %d, want %d", got.DurationMs, rec.DurationMs)
	}
	if got.CaseCount != rec.CaseCount {
		t.Errorf("CaseCount: got %d, want %d", got.CaseCount, rec.CaseCount)
	}
	if got.ErrorCount != rec.ErrorCount {
		t.Errorf("ErrorCount: got %d, want %d", got.ErrorCount, rec.ErrorCount)
	}
	if got.TraceEvents != rec.TraceEvents {
		t.Errorf("TraceEvents: got %d, want %d", got.TraceEvents, rec.TraceEvents)
	}
}

func TestRunRecord_LoadMissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadRunRecord(dir)
	if err == nil {
		t.Fatal("expected error for missing run.json, got nil")
	}
}

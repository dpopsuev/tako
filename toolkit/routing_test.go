package toolkit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoutingEntry_Fields(t *testing.T) {
	t.Parallel()
	e := RoutingEntry{
		CaseID:     "case-1",
		Step:       "triage",
		Color:      "green",
		Timestamp:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DispatchID: 42,
	}
	if e.CaseID != "case-1" || e.Step != "triage" || e.Color != "green" {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestRoutingLog_ForCase(t *testing.T) {
	t.Parallel()
	log := RoutingLog{
		{CaseID: "a", Step: "s1"},
		{CaseID: "b", Step: "s2"},
		{CaseID: "a", Step: "s3"},
	}
	filtered := log.ForCase("a")
	if len(filtered) != 2 {
		t.Fatalf("ForCase(a) len = %d, want 2", len(filtered))
	}
	if filtered[0].Step != "s1" || filtered[1].Step != "s3" {
		t.Errorf("ForCase steps = %v", filtered)
	}
}

func TestRoutingLog_ForStep(t *testing.T) {
	t.Parallel()
	log := RoutingLog{
		{CaseID: "a", Step: "triage"},
		{CaseID: "b", Step: "recall"},
		{CaseID: "c", Step: "triage"},
	}
	filtered := log.ForStep("triage")
	if len(filtered) != 2 {
		t.Fatalf("ForStep(triage) len = %d, want 2", len(filtered))
	}
}

func TestRoutingLog_ForCase_Empty(t *testing.T) {
	t.Parallel()
	log := RoutingLog{{CaseID: "a", Step: "s1"}}
	filtered := log.ForCase("missing")
	if len(filtered) != 0 {
		t.Errorf("expected empty, got %v", filtered)
	}
}

func TestRoutingLog_Len(t *testing.T) {
	t.Parallel()
	log := RoutingLog{{}, {}, {}}
	if log.Len() != 3 {
		t.Errorf("Len() = %d, want 3", log.Len())
	}
	var empty RoutingLog
	if empty.Len() != 0 {
		t.Errorf("empty Len() = %d, want 0", empty.Len())
	}
}

func TestSaveAndLoadRoutingLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")

	original := RoutingLog{
		{CaseID: "c1", Step: "recall", Color: "green", Timestamp: time.Now().Truncate(time.Second)},
		{CaseID: "c1", Step: "triage", Color: "red", DispatchID: 2},
	}

	if err := SaveRoutingLog(path, original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadRoutingLog(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded len = %d, want 2", len(loaded))
	}
	if loaded[0].CaseID != "c1" || loaded[0].Color != "green" {
		t.Errorf("loaded[0] = %+v", loaded[0])
	}
	if loaded[1].DispatchID != 2 {
		t.Errorf("loaded[1].DispatchID = %d, want 2", loaded[1].DispatchID)
	}
}

func TestLoadRoutingLog_Missing(t *testing.T) {
	t.Parallel()
	_, err := LoadRoutingLog("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSaveRoutingLog_BadPath(t *testing.T) {
	t.Parallel()
	err := SaveRoutingLog("/nonexistent/dir/file.json", RoutingLog{})
	if err == nil {
		t.Error("expected error for bad path")
	}
}

func TestLoadRoutingLog_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid"), 0644)
	_, err := LoadRoutingLog(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCompareRoutingLogs_NoDiffs(t *testing.T) {
	t.Parallel()
	a := RoutingLog{
		{CaseID: "c1", Step: "recall", Color: "green"},
		{CaseID: "c1", Step: "triage", Color: "red"},
	}
	diffs := CompareRoutingLogs(a, a)
	if len(diffs) != 0 {
		t.Errorf("identical logs should have no diffs, got %v", diffs)
	}
}

func TestCompareRoutingLogs_ColorMismatch(t *testing.T) {
	t.Parallel()
	expected := RoutingLog{{CaseID: "c1", Step: "recall", Color: "green"}}
	actual := RoutingLog{{CaseID: "c1", Step: "recall", Color: "red"}}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Expected != "green" || diffs[0].Actual != "red" {
		t.Errorf("diff = %+v", diffs[0])
	}
}

func TestCompareRoutingLogs_MissingInActual(t *testing.T) {
	t.Parallel()
	expected := RoutingLog{{CaseID: "c1", Step: "recall", Color: "green"}}
	actual := RoutingLog{}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Actual != "<missing>" {
		t.Errorf("diff.Actual = %q, want <missing>", diffs[0].Actual)
	}
}

func TestCompareRoutingLogs_ExtraInActual(t *testing.T) {
	t.Parallel()
	expected := RoutingLog{}
	actual := RoutingLog{{CaseID: "c1", Step: "recall", Color: "green"}}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Expected != "<missing>" {
		t.Errorf("diff.Expected = %q, want <missing>", diffs[0].Expected)
	}
}

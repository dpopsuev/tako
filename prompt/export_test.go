package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExport(t *testing.T) {
	store := NewLiveStore()
	store.Create("triage", "f1", "# Triage\n\nClassify the failure.")
	store.Create("investigate/deep-rca", "f3", "# Deep RCA\n\nFind root cause.")

	dir := t.TempDir()
	count, err := Export(store, dir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 exported, got %d", count)
	}

	// Verify files exist.
	data, err := os.ReadFile(filepath.Join(dir, "triage.md"))
	if err != nil {
		t.Fatalf("read triage.md: %v", err)
	}
	if string(data) != "# Triage\n\nClassify the failure." {
		t.Errorf("triage content = %q", string(data))
	}

	// Verify nested path.
	data, err = os.ReadFile(filepath.Join(dir, "investigate", "deep-rca.md"))
	if err != nil {
		t.Fatalf("read investigate/deep-rca.md: %v", err)
	}
	if string(data) != "# Deep RCA\n\nFind root cause." {
		t.Errorf("deep-rca content = %q", string(data))
	}
}

func TestExport_EmptyStore(t *testing.T) {
	store := NewLiveStore()
	dir := t.TempDir()
	count, err := Export(store, dir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

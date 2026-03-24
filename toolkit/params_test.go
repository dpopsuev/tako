package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPriorArtifacts_EmptyDir(t *testing.T) {
	t.Parallel()
	got := LoadPriorArtifacts("", []string{"a"}, func(n string) string { return n + ".json" })
	if got != nil {
		t.Errorf("empty caseDir should return nil, got %v", got)
	}
}

func TestLoadPriorArtifacts_NoFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := LoadPriorArtifacts(dir, []string{"a", "b"}, func(n string) string { return n + ".json" })
	if got != nil {
		t.Errorf("no files should return nil, got %v", got)
	}
}

func TestLoadPriorArtifacts_MixedPresence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	data := map[string]any{"status": "pass"}
	raw, _ := json.Marshal(data)
	os.WriteFile(filepath.Join(dir, "recall-result.json"), raw, 0644)

	got := LoadPriorArtifacts(dir, []string{"recall", "triage"}, func(n string) string {
		return NodeArtifactFilename(n, nil)
	})
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got["recall"] == nil || got["recall"]["status"] != "pass" {
		t.Errorf("recall artifact not loaded correctly: %v", got["recall"])
	}
	if got["triage"] != nil {
		t.Errorf("missing triage should not be in result, got %v", got["triage"])
	}
}

func TestLoadPriorArtifacts_EmptyFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	data := map[string]any{"ok": true}
	raw, _ := json.Marshal(data)
	os.WriteFile(filepath.Join(dir, "a-result.json"), raw, 0644)

	got := LoadPriorArtifacts(dir, []string{"a", "skip"}, func(n string) string {
		if n == "skip" {
			return ""
		}
		return n + "-result.json"
	})
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if _, ok := got["skip"]; ok {
		t.Error("node with empty filename should be skipped")
	}
	if got["a"] == nil {
		t.Error("node 'a' should be loaded")
	}
}

package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCaseDir(t *testing.T) {
	t.Parallel()
	got := CaseDir("/data", 10, 42)
	want := filepath.Join("/data", "10", "42")
	if got != want {
		t.Errorf("CaseDir = %q, want %q", got, want)
	}
}

func TestEnsureCaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	dir, err := EnsureCaseDir(base, 1, 2)
	if err != nil {
		t.Fatalf("EnsureCaseDir error: %v", err)
	}
	want := filepath.Join(base, "1", "2")
	if dir != want {
		t.Errorf("EnsureCaseDir = %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}

	dir2, err := EnsureCaseDir(base, 1, 2)
	if err != nil {
		t.Fatalf("idempotent call error: %v", err)
	}
	if dir2 != dir {
		t.Errorf("idempotent call returned %q, want %q", dir2, dir)
	}
}

func TestListCaseDirs(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	dirs, err := ListCaseDirs(base, 999)
	if err != nil {
		t.Fatalf("ListCaseDirs on non-existent suite: %v", err)
	}
	if dirs != nil {
		t.Errorf("expected nil for non-existent suite, got %v", dirs)
	}

	_, _ = EnsureCaseDir(base, 5, 10)
	_, _ = EnsureCaseDir(base, 5, 20)
	os.WriteFile(filepath.Join(base, "5", "not-a-dir.txt"), []byte("x"), 0644)

	dirs, err = ListCaseDirs(base, 5)
	if err != nil {
		t.Fatalf("ListCaseDirs error: %v", err)
	}
	if len(dirs) != 2 {
		t.Errorf("ListCaseDirs count = %d, want 2 (files excluded)", len(dirs))
	}
}

func TestNodeArtifactFilename(t *testing.T) {
	t.Parallel()

	if got := NodeArtifactFilename("recall", nil); got != "recall-result.json" {
		t.Errorf("convention fallback = %q, want recall-result.json", got)
	}

	overrides := map[string]string{"investigate": "artifact.json"}
	if got := NodeArtifactFilename("investigate", overrides); got != "artifact.json" {
		t.Errorf("override = %q, want artifact.json", got)
	}
	if got := NodeArtifactFilename("recall", overrides); got != "recall-result.json" {
		t.Errorf("not in overrides = %q, want recall-result.json", got)
	}
}

func TestNodePromptFilename(t *testing.T) {
	t.Parallel()

	if got := NodePromptFilename("", 0); got != "" {
		t.Errorf("empty node = %q, want empty", got)
	}
	if got := NodePromptFilename("triage", 0); got != "prompt-triage.md" {
		t.Errorf("iter=0: %q, want prompt-triage.md", got)
	}
	if got := NodePromptFilename("triage", 2); got != "prompt-triage-loop-2.md" {
		t.Errorf("iter=2: %q, want prompt-triage-loop-2.md", got)
	}
}

func TestReadMapArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, err := ReadMapArtifact(dir, "absent.json")
	if err != nil {
		t.Fatalf("missing file should return nil,nil: %v", err)
	}
	if got != nil {
		t.Errorf("missing file should return nil map, got %v", got)
	}

	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0644)
	_, err = ReadMapArtifact(dir, "bad.json")
	if err == nil {
		t.Error("invalid JSON should return error")
	}

	payload := map[string]any{"status": "pass", "score": float64(0.95)}
	raw, _ := json.Marshal(payload)
	os.WriteFile(filepath.Join(dir, "good.json"), raw, 0644)
	got, err = ReadMapArtifact(dir, "good.json")
	if err != nil {
		t.Fatalf("valid JSON error: %v", err)
	}
	if got["status"] != "pass" {
		t.Errorf("got[status] = %v, want pass", got["status"])
	}
}

func TestWriteArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	data := map[string]any{"key": "value", "num": float64(1)}
	if err := WriteArtifact(dir, "out.json", data); err != nil {
		t.Fatalf("WriteArtifact error: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "out.json"))
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	var roundtrip map[string]any
	if err := json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if roundtrip["key"] != "value" {
		t.Errorf("roundtrip[key] = %v, want value", roundtrip["key"])
	}
}

func TestWriteArtifact_Unmarshalable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := WriteArtifact(dir, "bad.json", make(chan int))
	if err == nil {
		t.Error("expected error for unmarshalable value")
	}
}

func TestWriteNodePrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path, err := WriteNodePrompt(dir, "recall", 0, "hello prompt")
	if err != nil {
		t.Fatalf("WriteNodePrompt error: %v", err)
	}
	want := filepath.Join(dir, "prompt-recall.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello prompt" {
		t.Errorf("content = %q, want %q", string(data), "hello prompt")
	}

	_, err = WriteNodePrompt(dir, "", 0, "content")
	if err == nil {
		t.Error("expected error for empty node name")
	}
}

func TestWriteAndReadArtifact_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	original := map[string]any{
		"status":     "pass",
		"confidence": float64(0.87),
		"tags":       []any{"a", "b"},
	}

	if err := WriteArtifact(dir, "test-result.json", original); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadMapArtifact(dir, "test-result.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got["status"] != "pass" || got["confidence"] != 0.87 {
		t.Errorf("roundtrip mismatch: %v", got)
	}
}

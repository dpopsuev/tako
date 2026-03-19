package calibrate_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dpopsuev/origami/calibrate"
)

func TestWriteAndReadManifest(t *testing.T) {
	dir := t.TempDir()

	want := &calibrate.Manifest{
		SchemaVersion: calibrate.SchemaV1,
		Schematic:     "gnd",
		CapturedAt:    time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Repos: []calibrate.RepoEntry{
			{Name: "my-repo", Branch: "main", SHA: "abc123", Files: []string{"go.mod", "main.go"}},
		},
		Docs: []calibrate.DocEntry{
			{Name: "arch", LocalPath: "docs/arch.md", SHA: "def456"},
		},
	}

	if err := calibrate.WriteManifest(dir, want); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := calibrate.ReadManifest(os.DirFS(dir))
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, want.SchemaVersion)
	}
	if got.Schematic != want.Schematic {
		t.Errorf("Schematic = %q, want %q", got.Schematic, want.Schematic)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("Repos count = %d, want 1", len(got.Repos))
	}
	if got.Repos[0].Name != "my-repo" {
		t.Errorf("Repo.Name = %q, want %q", got.Repos[0].Name, "my-repo")
	}
	if got.Repos[0].SHA != "abc123" {
		t.Errorf("Repo.SHA = %q, want %q", got.Repos[0].SHA, "abc123")
	}
	if len(got.Repos[0].Files) != 2 {
		t.Errorf("Repo.Files count = %d, want 2", len(got.Repos[0].Files))
	}
	if len(got.Docs) != 1 {
		t.Fatalf("Docs count = %d, want 1", len(got.Docs))
	}
	if got.Docs[0].SHA != "def456" {
		t.Errorf("Doc.SHA = %q, want %q", got.Docs[0].SHA, "def456")
	}
}

func TestReadManifest_NotFound(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := calibrate.ReadManifest(fsys)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestValidateBundle_AllPresent(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yaml": &fstest.MapFile{Data: []byte(`
schema_version: v1
schematic: gnd
captured_at: "2026-03-10T12:00:00Z"
repos:
  - name: myrepo
    sha: abc
    files:
      - main.go
docs:
  - name: arch
    local_path: docs/arch.md
    sha: def
`)},
		"repos/myrepo/main.go": &fstest.MapFile{Data: []byte("package main")},
		"docs/arch.md":         &fstest.MapFile{Data: []byte("# Architecture")},
	}

	errs := calibrate.ValidateBundle(fsys, false)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateBundle_MissingFile(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yaml": &fstest.MapFile{Data: []byte(`
schema_version: v1
schematic: gnd
captured_at: "2026-03-10T12:00:00Z"
repos:
  - name: myrepo
    sha: abc
    files:
      - main.go
      - missing.go
`)},
		"repos/myrepo/main.go": &fstest.MapFile{Data: []byte("package main")},
	}

	errs := calibrate.ValidateBundle(fsys, false)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateBundle_SHAMismatch(t *testing.T) {
	content := []byte("# Architecture")
	sha := "wrong_sha"

	fsys := fstest.MapFS{
		"manifest.yaml": &fstest.MapFile{Data: []byte(`
schema_version: v1
schematic: gnd
captured_at: "2026-03-10T12:00:00Z"
docs:
  - name: arch
    local_path: docs/arch.md
    sha: ` + sha + `
`)},
		"docs/arch.md": &fstest.MapFile{Data: content},
	}

	errs := calibrate.ValidateBundle(fsys, true)
	if len(errs) != 1 {
		t.Errorf("expected 1 SHA mismatch error, got %d: %v", len(errs), errs)
	}
}

func TestFileChecksum(t *testing.T) {
	fsys := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: []byte("hello")},
	}

	sum, err := calibrate.FileChecksum(fsys, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Error("expected non-empty checksum")
	}

	sum2, _ := calibrate.FileChecksum(fsys, "test.txt")
	if sum != sum2 {
		t.Errorf("checksum not deterministic: %q vs %q", sum, sum2)
	}
}

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(sub, "b.txt"), []byte("b"), 0o644)

	files, err := calibrate.CollectFiles(os.DirFS(dir), ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != "a.txt" {
		t.Errorf("files[0] = %q, want %q", files[0], "a.txt")
	}
}

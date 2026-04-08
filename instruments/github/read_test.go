package github

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0o644)

	data, err := ReadFile(context.Background(), dir, "test.go")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "package main" {
		t.Errorf("got %q", data)
	}
}

func TestReadFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadFile(context.Background(), dir, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestReadFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadFile(context.Background(), dir, "nonexistent.go")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestListTree(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pkg", "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "pkg", "lib.go"), []byte("package pkg"), 0o644)
	os.WriteFile(filepath.Join(dir, "pkg", "sub", "deep.go"), []byte("package sub"), 0o644)

	entries, err := ListTree(context.Background(), dir, 3)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}

	paths := make(map[string]bool)
	for _, e := range entries {
		paths[e.Path] = true
	}

	for _, want := range []string{"main.go", "pkg", filepath.Join("pkg", "lib.go")} {
		if !paths[want] {
			t.Errorf("missing %q in tree: %v", want, paths)
		}
	}
}

func TestListTree_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	entries, err := ListTree(context.Background(), dir, 3)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}

	for _, e := range entries {
		if e.Path == ".git" || filepath.Dir(e.Path) == ".git" {
			t.Errorf("should skip .git: got %q", e.Path)
		}
	}
}

func TestListTree_MaxDepth(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b", "c", "d"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "b", "c", "d", "deep.go"), []byte("deep"), 0o644)

	entries, err := ListTree(context.Background(), dir, 2)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}

	for _, e := range entries {
		if e.Path == filepath.Join("a", "b", "c", "d", "deep.go") {
			t.Errorf("should not include files beyond maxDepth")
		}
	}
}

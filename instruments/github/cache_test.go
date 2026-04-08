package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoCache_RepoPath(t *testing.T) {
	base := filepath.Join("tmp", "cache")
	c := NewRepoCache(base, "")
	got := c.repoPath("openshift", "ptp-operator", "release-4.21")
	want := filepath.Join(base, "openshift", "ptp-operator", "release-4.21")
	if got != want {
		t.Errorf("repoPath = %q, want %q", got, want)
	}
}

func TestRepoCache_Clear(t *testing.T) {
	dir := t.TempDir()
	c := NewRepoCache(dir, "")
	sub := filepath.Join(dir, "openshift", "ptp-operator", "main")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "test.txt"), []byte("test"), 0o644)

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cache dir should be removed")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir := DefaultCacheDir()
	if dir == "" {
		t.Error("DefaultCacheDir returned empty")
	}
}

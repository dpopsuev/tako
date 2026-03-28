package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveToken_EnvVar(t *testing.T) {
	t.Setenv(tokenEnvVar, "env-token-123")
	tok, err := ResolveToken("")
	if err != nil {
		t.Fatalf("ResolveToken: %v", err)
	}
	if tok != "env-token-123" {
		t.Errorf("got %q, want env-token-123", tok)
	}
}

func TestResolveToken_File(t *testing.T) {
	t.Setenv(tokenEnvVar, "")
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	os.WriteFile(tokenFile, []byte("file-token-456\n"), 0o600)

	tok, err := ResolveToken(tokenFile)
	if err != nil {
		t.Fatalf("ResolveToken: %v", err)
	}
	if tok != "file-token-456" {
		t.Errorf("got %q, want file-token-456", tok)
	}
}

func TestResolveToken_None(t *testing.T) {
	t.Setenv(tokenEnvVar, "")
	tok, err := ResolveToken("/nonexistent/path")
	if err != nil {
		t.Fatalf("ResolveToken: %v", err)
	}
	if tok != "" {
		t.Errorf("got %q, want empty", tok)
	}
}

func TestCloneURL_WithToken(t *testing.T) {
	got := cloneURL("openshift", "ptp-operator", "my-token")
	want := "https://my-token@github.com/openshift/ptp-operator.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCloneURL_WithoutToken(t *testing.T) {
	got := cloneURL("openshift", "ptp-operator", "")
	want := "https://github.com/openshift/ptp-operator.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

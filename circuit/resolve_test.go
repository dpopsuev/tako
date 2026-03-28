package circuit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCircuitPath_Embedded(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	content := []byte("circuit: test\nnodes: []\nedges: []")
	RegisterEmbeddedCircuit("myCircuit", content)

	got, err := ResolveCircuitPath("mycircuit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestResolveCircuitPath_EmbeddedCaseInsensitive(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	content := []byte("circuit: ci")
	RegisterEmbeddedCircuit("CI-Circuit", content)

	got, err := ResolveCircuitPath("ci-circuit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch")
	}
}

func TestResolveCircuitPath_FilesystemFallback(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	dir := t.TempDir()
	content := []byte("circuit: fs-test\nnodes: []\nedges: []")
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveCircuitPath("test", WithSearchDirs(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch")
	}
}

func TestResolveCircuitPath_EnvVar(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	dir := t.TempDir()
	content := []byte("circuit: env-test")
	if err := os.WriteFile(filepath.Join(dir, "envpipe.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ORIGAMI_CIRCUITS", dir)

	got, err := ResolveCircuitPath("envpipe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch")
	}
}

func TestResolveCircuitPath_NotFound(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	_, err := ResolveCircuitPath("nonexistent-circuit-xyz")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "not found") {
		t.Fatalf("error should mention 'not found': %s", got)
	}
	if got := err.Error(); !strings.Contains(got, "searched:") {
		t.Fatalf("error should list searched paths: %s", got)
	}
}

func TestResolveCircuitPath_AutoYamlSuffix(t *testing.T) {
	ClearEmbeddedCircuits()
	defer ClearEmbeddedCircuits()

	dir := t.TempDir()
	content := []byte("circuit: suffix-test")
	if err := os.WriteFile(filepath.Join(dir, "myfile.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveCircuitPath("myfile", WithSearchDirs(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch")
	}
}

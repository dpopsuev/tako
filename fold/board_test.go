package fold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBoardManifest_AllFields(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "manifests", "board-flat.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	bm, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("ParseBoardManifest: %v", err)
	}

	if bm.Name != "test-board" {
		t.Errorf("Name = %q, want test-board", bm.Name)
	}
	if len(bm.Uses) != 2 {
		t.Errorf("Uses count = %d, want 2", len(bm.Uses))
	}
	if bm.Uses["rca"] != "github.com/dpopsuev/origami-rca" {
		t.Errorf("Uses[rca] = %q", bm.Uses["rca"])
	}
	if len(bm.Bind) != 1 {
		t.Errorf("Bind count = %d, want 1", len(bm.Bind))
	}
	if bm.Domain != "../domains/ocp/ptp" {
		t.Errorf("Domain = %q", bm.Domain)
	}
	if len(bm.Prompts) != 2 {
		t.Errorf("Prompts count = %d, want 2", len(bm.Prompts))
	}
	if bm.Circuit == nil {
		t.Fatal("Circuit is nil")
	}
	if len(bm.Circuit.Import) != 2 {
		t.Errorf("Circuit.Import count = %d, want 2", len(bm.Circuit.Import))
	}
	if bm.Serve == nil || bm.Serve.Port != 9300 {
		t.Errorf("Serve.Port = %v", bm.Serve)
	}
}

func TestParseBoardManifest_MissingName(t *testing.T) {
	data := []byte("kind: Board\n")
	_, err := ParseBoardManifest(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !errors.Is(err, ErrBoardNameRequired) {
		t.Errorf("error = %v, want ErrBoardNameRequired", err)
	}
}

func TestParseBoardManifest_WrongKind(t *testing.T) {
	data := []byte("kind: Scorecard\nname: test\n")
	_, err := ParseBoardManifest(data)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
	if !errors.Is(err, ErrBoardKindMismatch) {
		t.Errorf("error = %v, want ErrBoardKindMismatch", err)
	}
}

func TestParseBoardManifest_MinimalValid(t *testing.T) {
	data := []byte("kind: Board\nname: minimal\n")
	bm, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("ParseBoardManifest: %v", err)
	}
	if bm.Name != "minimal" {
		t.Errorf("Name = %q", bm.Name)
	}
}

func TestParseBoardManifest_WithCompose(t *testing.T) {
	data := []byte("kind: Board\nname: child\ncompose:\n  base: ./parent.yaml\n")
	bm, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("ParseBoardManifest: %v", err)
	}
	if bm.Compose == nil || bm.Compose.Base != "./parent.yaml" {
		t.Errorf("Compose = %v", bm.Compose)
	}
}

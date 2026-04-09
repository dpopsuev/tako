package fold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestComposition_MergesUsesFromBase(t *testing.T) {
	dir := filepath.Join("testdata", "manifests")
	data, err := os.ReadFile(filepath.Join(dir, "board-compose-child.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	child, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	merged, err := ResolveBoardComposition(child, dir)
	if err != nil {
		t.Fatalf("ResolveBoardComposition: %v", err)
	}

	// Child's uses + base's uses.
	if len(merged.Uses) != 3 { //nolint:mnd // rca + gnd from base + tuner from child
		t.Errorf("Uses count = %d, want 3", len(merged.Uses))
	}
	if merged.Uses["rca"] == "" {
		t.Error("missing rca from base")
	}
	if merged.Uses["tuner"] == "" {
		t.Error("missing tuner from child")
	}
}

func TestComposition_ChildOverridesServePort(t *testing.T) {
	dir := filepath.Join("testdata", "manifests")
	data, err := os.ReadFile(filepath.Join(dir, "board-compose-child.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	child, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	merged, err := ResolveBoardComposition(child, dir)
	if err != nil {
		t.Fatalf("ResolveBoardComposition: %v", err)
	}

	if merged.Serve == nil || merged.Serve.Port != 9400 { //nolint:mnd // child overrides
		t.Errorf("Serve.Port = %v, want 9400", merged.Serve)
	}
}

func TestComposition_ChildName(t *testing.T) {
	dir := filepath.Join("testdata", "manifests")
	data, err := os.ReadFile(filepath.Join(dir, "board-compose-child.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	child, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	merged, err := ResolveBoardComposition(child, dir)
	if err != nil {
		t.Fatalf("ResolveBoardComposition: %v", err)
	}

	if merged.Name != "child-board" {
		t.Errorf("Name = %q, want child-board", merged.Name)
	}
}

func TestComposition_CycleDetection(t *testing.T) {
	dir := filepath.Join("testdata", "manifests")
	data, err := os.ReadFile(filepath.Join(dir, "board-compose-cycle-a.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	bm, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = ResolveBoardComposition(bm, dir)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !errors.Is(err, ErrCompositionCycle) {
		t.Errorf("error = %v, want ErrCompositionCycle", err)
	}
}

func TestComposition_NoCompose_PassThrough(t *testing.T) {
	data := []byte("kind: Board\nname: standalone\n")
	bm, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := ResolveBoardComposition(bm, ".")
	if err != nil {
		t.Fatalf("ResolveBoardComposition: %v", err)
	}
	if result.Name != "standalone" {
		t.Errorf("Name = %q, want standalone", result.Name)
	}
	if result.Compose != nil {
		t.Error("Compose should be nil after resolution")
	}
}

func TestComposition_BaseBindInherited(t *testing.T) {
	dir := filepath.Join("testdata", "manifests")
	data, err := os.ReadFile(filepath.Join(dir, "board-compose-child.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	child, err := ParseBoardManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	merged, err := ResolveBoardComposition(child, dir)
	if err != nil {
		t.Fatalf("ResolveBoardComposition: %v", err)
	}

	if merged.Bind["rca.source"] == "" {
		t.Error("bind rca.source should be inherited from base")
	}
}

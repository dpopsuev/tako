package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTuneCmd_Preflight(t *testing.T) {
	dir := t.TempDir()

	// Copy dummy-echo instrument manifest.
	srcManifest := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "instrument.yaml")
	instDir := filepath.Join(dir, "instruments", "dummy-echo")
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, srcManifest, filepath.Join(instDir, "instrument.yaml"))

	// Copy the echo.sh script.
	srcScript := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "echo.sh")
	copyFile(t, srcScript, filepath.Join(instDir, "echo.sh"))
	os.Chmod(filepath.Join(instDir, "echo.sh"), 0o755)

	// Create board manifest.
	board := `kind: Board
name: tune-test
instruments:
  dummy-echo: instruments/dummy-echo/instrument.yaml
`
	boardPath := filepath.Join(dir, "board.yaml")
	os.WriteFile(boardPath, []byte(board), 0o600)

	// Run tune (no --sum).
	err := tuneCmd([]string{boardPath})
	if err != nil {
		t.Fatalf("tuneCmd: %v", err)
	}
}

func TestTuneCmd_Sum_WritesChecksum(t *testing.T) {
	dir := t.TempDir()

	// Copy dummy-echo instrument manifest.
	srcManifest := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "instrument.yaml")
	instDir := filepath.Join(dir, "instruments", "dummy-echo")
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	instManifest := filepath.Join(instDir, "instrument.yaml")
	copyFile(t, srcManifest, instManifest)

	// Copy the echo.sh script.
	srcScript := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "echo.sh")
	copyFile(t, srcScript, filepath.Join(instDir, "echo.sh"))
	os.Chmod(filepath.Join(instDir, "echo.sh"), 0o755)

	// Create board manifest.
	board := `kind: Board
name: tune-test
instruments:
  dummy-echo: instruments/dummy-echo/instrument.yaml
`
	boardPath := filepath.Join(dir, "board.yaml")
	os.WriteFile(boardPath, []byte(board), 0o600)

	// Run tune --sum.
	err := tuneCmd([]string{"--sum", boardPath})
	if err != nil {
		t.Fatalf("tuneCmd --sum: %v", err)
	}

	// Verify checksum was written to the instrument manifest.
	data, err := os.ReadFile(instManifest)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "checksum:") {
		t.Errorf("instrument manifest should contain checksum after --sum:\n%s", content)
	}
	if !strings.Contains(content, "sha256:") {
		t.Errorf("checksum should be sha256 format:\n%s", content)
	}
}

func TestTuneCmd_NoInstruments(t *testing.T) {
	dir := t.TempDir()
	board := `kind: Board
name: empty-test
`
	boardPath := filepath.Join(dir, "board.yaml")
	os.WriteFile(boardPath, []byte(board), 0o600)

	// Should succeed with no error (just prints "no instruments").
	err := tuneCmd([]string{boardPath})
	if err != nil {
		t.Fatalf("tuneCmd with no instruments: %v", err)
	}
}

func TestTuneCmd_Sum_Idempotent(t *testing.T) {
	dir := t.TempDir()

	srcManifest := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "instrument.yaml")
	instDir := filepath.Join(dir, "instruments", "dummy-echo")
	os.MkdirAll(instDir, 0o755)
	instManifest := filepath.Join(instDir, "instrument.yaml")
	copyFile(t, srcManifest, instManifest)

	srcScript := filepath.Join(repoRootFromCmd(), "testkit", "instruments", "dummy-echo", "echo.sh")
	copyFile(t, srcScript, filepath.Join(instDir, "echo.sh"))
	os.Chmod(filepath.Join(instDir, "echo.sh"), 0o755)

	board := `kind: Board
name: tune-test
instruments:
  dummy-echo: instruments/dummy-echo/instrument.yaml
`
	boardPath := filepath.Join(dir, "board.yaml")
	os.WriteFile(boardPath, []byte(board), 0o600)

	// First run writes checksum.
	tuneCmd([]string{"--sum", boardPath})

	// Read manifest after first run.
	data1, _ := os.ReadFile(instManifest)

	// Second run should be idempotent (no change).
	tuneCmd([]string{"--sum", boardPath})

	data2, _ := os.ReadFile(instManifest)
	if !bytes.Equal(data1, data2) {
		t.Error("--sum should be idempotent — second run changed the manifest")
	}
}

// --- helpers ---

func repoRootFromCmd() string {
	// cmd/origami/ is two levels deep from repo root.
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

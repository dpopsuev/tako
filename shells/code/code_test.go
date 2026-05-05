package code

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/agent/shell"
)

func capSet(dir string) *shell.CapabilitySet {
	cs := shell.NewCapabilitySet()
	for _, c := range Capabilities(dir) {
		cs.Register(c)
	}
	return cs
}

func TestCodeCapabilities_Names(t *testing.T) {
	cs := capSet(t.TempDir())
	names := cs.Names()
	expected := []string{"read_file", "write_file", "edit", "bash", "glob", "grep", "git_status", "git_diff", "git_commit", "go_build", "go_test", "go_vet"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("names[%d] = %s, want %s", i, names[i], name)
		}
	}
}

func TestCodeCapabilities_Modes(t *testing.T) {
	cs := capSet(t.TempDir())
	check := func(name string, want shell.ActionMode) {
		cap, ok := cs.Get(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		if cap.Mode != want {
			t.Errorf("%s mode = %v, want %v", name, cap.Mode, want)
		}
	}
	check("read_file", shell.ReadAction)
	check("write_file", shell.WriteAction)
	check("go_test", shell.WriteAction)
}

func TestCodeCapabilities_Risk(t *testing.T) {
	cs := capSet(t.TempDir())
	check := func(name string, want float64) {
		cap, _ := cs.Get(name)
		if cap.Risk != want {
			t.Errorf("%s risk = %f, want %f", name, cap.Risk, want)
		}
	}
	check("read_file", 0)
	check("write_file", 0.7)
	check("go_test", 0.3)
}

func TestCodeCapabilities_ReadFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644)

	cs := capSet(dir)
	cap, _ := cs.Get("read_file")
	result, err := cap.Execute(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Text()) != "world" {
		t.Errorf("got %q, want %q", string(result.Text()), "world")
	}
}

func TestCodeCapabilities_ReadFile_Escape(t *testing.T) {
	cs := capSet(t.TempDir())
	cap, _ := cs.Get("read_file")
	result, _ := cap.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd"}`))
	if !result.IsError {
		t.Error("expected error for path escape")
	}
}

func TestCodeCapabilities_WriteFile(t *testing.T) {
	dir := t.TempDir()
	cs := capSet(dir)
	cap, _ := cs.Get("write_file")

	result, err := cap.Execute(context.Background(), json.RawMessage(`{"path":"sub/test.go","content":"package sub\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", string(result.Text()))
	}

	data, err := os.ReadFile(filepath.Join(dir, "sub", "test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package sub\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestCodeCapabilities_WriteFile_Escape(t *testing.T) {
	cs := capSet(t.TempDir())
	cap, _ := cs.Get("write_file")
	result, _ := cap.Execute(context.Background(), json.RawMessage(`{"path":"../../tmp/evil","content":"bad"}`))
	if !result.IsError {
		t.Error("expected error for path escape")
	}
}

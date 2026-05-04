package code

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/agent/shell"
)

func TestCodeShell_Names(t *testing.T) {
	sh := NewShell(t.TempDir())
	names := sh.Names()
	expected := []string{"read_file", "write_file", "go_build", "go_test", "go_vet"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("names[%d] = %s, want %s", i, names[i], name)
		}
	}
}

func TestCodeShell_Modes(t *testing.T) {
	sh := NewShell(t.TempDir())
	if m := sh.Mode("read_file"); m != shell.ReadAction {
		t.Errorf("read_file mode = %v, want ReadAction", m)
	}
	if m := sh.Mode("write_file"); m != shell.WriteAction {
		t.Errorf("write_file mode = %v, want WriteAction", m)
	}
	if m := sh.Mode("go_test"); m != shell.WriteAction {
		t.Errorf("go_test mode = %v, want WriteAction", m)
	}
}

func TestCodeShell_Risk(t *testing.T) {
	sh := NewShell(t.TempDir())
	if r := sh.Risk("read_file"); r != 0 {
		t.Errorf("read_file risk = %f, want 0", r)
	}
	if r := sh.Risk("write_file"); r != 0.7 {
		t.Errorf("write_file risk = %f, want 0.7", r)
	}
	if r := sh.Risk("go_test"); r != 0.3 {
		t.Errorf("go_test risk = %f, want 0.3", r)
	}
}

func TestCodeShell_ReadFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644)

	sh := NewShell(dir)
	result, err := sh.Exec(context.Background(), "read_file", json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Text()) != "world" {
		t.Errorf("got %q, want %q", string(result.Text()), "world")
	}
}

func TestCodeShell_ReadFile_Escape(t *testing.T) {
	sh := NewShell(t.TempDir())
	result, _ := sh.Exec(context.Background(), "read_file", json.RawMessage(`{"path":"../../etc/passwd"}`))
	if !result.IsError {
		t.Error("expected error for path escape")
	}
}

func TestCodeShell_WriteFile(t *testing.T) {
	dir := t.TempDir()
	sh := NewShell(dir)

	result, err := sh.Exec(context.Background(), "write_file", json.RawMessage(`{"path":"sub/test.go","content":"package sub\n"}`))
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

func TestCodeShell_WriteFile_Escape(t *testing.T) {
	sh := NewShell(t.TempDir())
	result, _ := sh.Exec(context.Background(), "write_file", json.RawMessage(`{"path":"../../tmp/evil","content":"bad"}`))
	if !result.IsError {
		t.Error("expected error for path escape")
	}
}

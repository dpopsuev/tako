package instrument

import (
	"context"
	"errors"
	"testing"
)

func TestStubShellNames(t *testing.T) {
	shell := NewStubShell()
	names := shell.Names()
	if len(names) != 1 || names[0] != "echo" {
		t.Errorf("expected [echo], got %v", names)
	}
}

func TestStubShellExec(t *testing.T) {
	shell := NewStubShell()
	result, err := shell.Exec(context.Background(), "echo", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content) != "hello" {
		t.Errorf("expected echo of input, got %q", result.Content)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Structure != Blob {
		t.Errorf("expected Blob structure, got %d", result.Structure)
	}
}

func TestStubShellNotFound(t *testing.T) {
	shell := NewStubShell()
	_, err := shell.Exec(context.Background(), "nonexistent", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStubShellSignature(t *testing.T) {
	shell := NewStubShell()
	sig, err := shell.Signature("echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == "" {
		t.Error("expected non-empty signature")
	}
}

func TestStubShellManual(t *testing.T) {
	shell := NewStubShell()
	manual, err := shell.Manual("echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manual == "" {
		t.Error("expected non-empty manual")
	}
}

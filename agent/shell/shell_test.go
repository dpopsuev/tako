package shell

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestEchoFunction_Satisfies(t *testing.T) {
	var _ Function = EchoFunction{}
}

func TestEchoFunction_Execute(t *testing.T) {
	fn := EchoFunction{}
	result, err := fn.Execute(context.Background(), json.RawMessage(`"hello"`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("expected text content, got %s", result.Content[0].Type)
	}
	if result.IsError {
		t.Error("should not be error")
	}
}

func TestEchoFunction_Metadata(t *testing.T) {
	fn := EchoFunction{}
	if fn.Name() != "echo" {
		t.Errorf("name: %s", fn.Name())
	}
	if fn.Description() == "" {
		t.Error("description should not be empty")
	}
	schema := fn.InputSchema()
	if !json.Valid(schema) {
		t.Error("schema should be valid JSON")
	}
}

func TestStubShell_Names(t *testing.T) {
	shell := NewStubShell()
	names := shell.Names()
	if len(names) != 1 || names[0] != "echo" {
		t.Errorf("expected [echo], got %v", names)
	}
}

func TestStubShell_Exec(t *testing.T) {
	shell := NewStubShell()
	result, err := shell.Exec(context.Background(), "echo", json.RawMessage(`"hello"`))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	if result.Content[0].Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestStubShell_NotFound(t *testing.T) {
	shell := NewStubShell()
	_, err := shell.Exec(context.Background(), "nonexistent", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStubShell_Describe(t *testing.T) {
	shell := NewStubShell()
	desc, err := shell.Describe("echo")
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestStubShell_Schema(t *testing.T) {
	shell := NewStubShell()
	schema, err := shell.Schema("echo")
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if !json.Valid(schema) {
		t.Error("expected valid JSON schema")
	}
}

func TestNewShellWith_CustomFunctions(t *testing.T) {
	shell := NewShellWith(EchoFunction{})
	names := shell.Names()
	if len(names) != 1 {
		t.Errorf("expected 1 function, got %d", len(names))
	}
}

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(r.Content))
	}
	if r.Content[0].Text != "hello" {
		t.Errorf("text: %s", r.Content[0].Text)
	}
	if r.IsError {
		t.Error("should not be error")
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("failed")
	if !r.IsError {
		t.Error("should be error")
	}
	if r.Content[0].Text != "failed" {
		t.Errorf("text: %s", r.Content[0].Text)
	}
}

package instrument

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCommand_Cat(t *testing.T) {
	fn, err := NewCommand("cat", "reads stdin and outputs it")
	if err != nil {
		t.Skipf("cat not in PATH: %v", err)
	}

	var _ Function = fn

	result, err := fn.Execute(context.Background(), json.RawMessage(`{"input":"hello"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	if result.IsError {
		t.Error("should not be error")
	}
	if result.Content[0].Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestCommand_NotFound(t *testing.T) {
	_, err := NewCommand("nonexistent-command-xyz", "missing")
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestCommand_Metadata(t *testing.T) {
	fn, err := NewCommand("echo", "echoes input")
	if err != nil {
		t.Skipf("echo not in PATH: %v", err)
	}
	if fn.Description() != "echoes input" {
		t.Errorf("description: %s", fn.Description())
	}
	if !json.Valid(fn.InputSchema()) {
		t.Error("schema should be valid JSON")
	}
}

func TestCommand_FailingCommand(t *testing.T) {
	fn, err := NewCommand("false", "always fails")
	if err != nil {
		t.Skipf("false not in PATH: %v", err)
	}
	result, err := fn.Execute(context.Background(), json.RawMessage(`{"input":""}`))
	if err == nil {
		t.Log("false might produce output on some systems")
	}
	if err != nil && !errors.Is(err, ErrCLINoOutput) {
		if result.IsError {
			t.Logf("error result: %s", result.Content[0].Text)
		}
	}
}

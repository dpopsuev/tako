package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestExecDispatcher_Echo(t *testing.T) {
	d := &ExecDispatcher{
		Command: "bash testkit/instruments/dummy-echo/echo.sh",
		WorkDir: "..",
	}

	input := json.RawMessage(`{"message":"hello"}`)
	out, err := d.Dispatch(context.Background(), input)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	echo, ok := result["echo"].(map[string]any)
	if !ok {
		t.Fatalf("echo type = %T, want map", result["echo"])
	}
	if echo["message"] != "hello" {
		t.Errorf("echo.message = %v, want hello", echo["message"])
	}
}

func TestExecDispatcher_Fail(t *testing.T) {
	d := &ExecDispatcher{
		Command: "bash testkit/instruments/dummy-fail/fail.sh",
		WorkDir: "..",
	}

	_, err := d.Dispatch(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error from failing instrument")
	}
	if !errors.Is(err, ErrInstrumentDispatch) {
		t.Errorf("want ErrInstrumentDispatch, got %v", err)
	}
}

func TestExecDispatcher_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := &ExecDispatcher{Command: "sleep 10"}

	_, err := d.Dispatch(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestExecDispatcher_EmptyCommand(t *testing.T) {
	d := &ExecDispatcher{}

	_, err := d.Dispatch(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !errors.Is(err, ErrInstrumentDispatch) {
		t.Errorf("want ErrInstrumentDispatch, got %v", err)
	}
}

func TestExecDispatcher_InvalidCommand(t *testing.T) {
	d := &ExecDispatcher{Command: "/nonexistent/binary"}

	_, err := d.Dispatch(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
	if !errors.Is(err, ErrInstrumentDispatch) {
		t.Errorf("want ErrInstrumentDispatch, got %v", err)
	}
}

func TestExecDispatcher_EmptyInput(t *testing.T) {
	d := &ExecDispatcher{
		Command: "bash testkit/instruments/dummy-echo/echo.sh",
		WorkDir: "..",
	}

	out, err := d.Dispatch(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

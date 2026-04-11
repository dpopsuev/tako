package stubs_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

// Compile-time check: StubInstrumentDispatcher satisfies engine.InstrumentDispatcher.
var _ engine.InstrumentDispatcher = (*stubs.StubInstrumentDispatcher)(nil)

func TestStubInstrumentTool_Contract(t *testing.T) {
	contracts.RunInstrumentToolContract(t, func() tool.Tool {
		return stubs.NewStubInstrumentTool("test-scan", "test scanner")
	})
}

func TestStubInstrumentNode_Contract(t *testing.T) {
	contracts.RunInstrumentNodeContract(t, func() circuit.Node {
		tl := stubs.NewStubInstrumentTool("test-scan", "test scanner")
		return stubs.NewStubInstrumentNode(tl)
	})
}

func TestStubInstrumentTool_ErrorInjection(t *testing.T) {
	tl := stubs.NewStubInstrumentTool("scan", "scanner")
	injected := errors.New("connection refused")
	tl.SetError(injected)

	_, err := tl.Execute(t.Context(), nil)
	if !errors.Is(err, injected) {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestStubInstrumentTool_CallTracking(t *testing.T) {
	tl := stubs.NewStubInstrumentTool("scan", "scanner")
	tl.Execute(t.Context(), []byte(`{"repo":"."}`))
	tl.Execute(t.Context(), []byte(`{"repo":"/tmp"}`))

	if tl.CallCount() != 2 {
		t.Errorf("CallCount = %d, want 2", tl.CallCount())
	}
	calls := tl.Calls()
	if string(calls[0]) != `{"repo":"."}` {
		t.Errorf("calls[0] = %s", calls[0])
	}
}

func TestStubInstrumentDispatcher_CannedResult(t *testing.T) {
	want := json.RawMessage(`{"scan":"complete"}`)
	d := stubs.NewStubInstrumentDispatcher(want)

	got, err := d.Dispatch(t.Context(), json.RawMessage(`{"repo":"."}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Dispatch() = %s, want %s", got, want)
	}
}

func TestStubInstrumentDispatcher_DefaultResult(t *testing.T) {
	d := stubs.NewStubInstrumentDispatcher(nil)

	got, err := d.Dispatch(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("Dispatch() = %s, want default {\"ok\":true}", got)
	}
}

func TestStubInstrumentDispatcher_ErrorInjection(t *testing.T) {
	d := stubs.NewStubInstrumentDispatcher(nil)
	injected := errors.New("connection refused")
	d.SetError(injected)

	_, err := d.Dispatch(t.Context(), json.RawMessage(`{}`))
	if !errors.Is(err, injected) {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestStubInstrumentDispatcher_CallTracking(t *testing.T) {
	d := stubs.NewStubInstrumentDispatcher(nil)
	d.Dispatch(t.Context(), json.RawMessage(`{"a":1}`))
	d.Dispatch(t.Context(), json.RawMessage(`{"b":2}`))

	if d.CallCount() != 2 {
		t.Errorf("CallCount = %d, want 2", d.CallCount())
	}
	calls := d.Calls()
	if string(calls[0]) != `{"a":1}` {
		t.Errorf("calls[0] = %s, want {\"a\":1}", calls[0])
	}
	if string(calls[1]) != `{"b":2}` {
		t.Errorf("calls[1] = %s, want {\"b\":2}", calls[1])
	}
}

func TestStubInstrumentDispatcher_SetResult(t *testing.T) {
	d := stubs.NewStubInstrumentDispatcher(nil)
	d.SetResult(json.RawMessage(`{"updated":true}`))

	got, err := d.Dispatch(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"updated":true}` {
		t.Errorf("Dispatch() = %s, want {\"updated\":true}", got)
	}
}

func TestStubInstrumentDispatcher_Reset(t *testing.T) {
	d := stubs.NewStubInstrumentDispatcher(nil)
	d.SetError(errors.New("fail"))
	d.Dispatch(t.Context(), json.RawMessage(`{}`))
	d.Reset()

	if d.CallCount() != 0 {
		t.Errorf("CallCount after Reset = %d, want 0", d.CallCount())
	}

	got, err := d.Dispatch(t.Context(), json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("unexpected error after Reset: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("Dispatch() after Reset = %s, want default", got)
	}
}

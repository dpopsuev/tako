package stubs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/origami/testkit"
)

func TestStubTransport_SatisfiesTransportInterface(t *testing.T) {
	var _ testkit.Transport = (*StubTransport)(nil)
}

func TestStubTrigger_SatisfiesTriggerInterface(t *testing.T) {
	var _ testkit.Trigger = (*StubTrigger)(nil)
}

func TestStubTransport_Serve_BlocksUntilContextCancel(t *testing.T) {
	st := NewStubTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := st.Serve(ctx, nil)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Serve: %v", err)
	}

	calls := st.Calls()
	if len(calls) != 1 || calls[0] != "Serve" {
		t.Errorf("Calls = %v, want [Serve]", calls)
	}
}

func TestStubTransport_Shutdown_TracksCall(t *testing.T) {
	st := NewStubTransport()
	if err := st.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	calls := st.Calls()
	if len(calls) != 1 || calls[0] != "Shutdown" {
		t.Errorf("Calls = %v, want [Shutdown]", calls)
	}
}

func TestStubTransport_SetError_InjectsError(t *testing.T) {
	st := NewStubTransport()
	injected := &testError{"serve-error"}
	st.SetError(injected)

	err := st.Serve(context.Background(), nil)
	if !errors.Is(err, injected) {
		t.Errorf("Serve error = %v, want %v", err, injected)
	}
}

func TestStubTransport_Reset_ClearsState(t *testing.T) {
	st := NewStubTransport()
	st.SetError(&testError{"boom"})
	_ = st.Shutdown(context.Background())

	st.Reset()

	if len(st.Calls()) != 0 {
		t.Errorf("Calls after Reset = %d, want 0", len(st.Calls()))
	}
}

func TestStubTrigger_Start_ReturnsCannedHandle(t *testing.T) {
	tr := NewStubTrigger()
	handle := &StubSessionHandle{id: "test-session"}
	tr.WithHandle(handle)

	got, err := tr.Start(context.Background(), testkit.TriggerParams{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got.ID() != "test-session" {
		t.Errorf("ID = %q, want %q", got.ID(), "test-session")
	}
}

func TestStubTrigger_Start_TracksParams(t *testing.T) {
	tr := NewStubTrigger()
	tr.WithHandle(&StubSessionHandle{id: "s1"})

	params := testkit.TriggerParams{Parallel: 4, Force: true}
	_, err := tr.Start(context.Background(), params)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(tr.Calls()) != 1 {
		t.Fatalf("Calls = %d, want 1", len(tr.Calls()))
	}
	if tr.LastParams().Parallel != 4 {
		t.Errorf("LastParams.Parallel = %d, want 4", tr.LastParams().Parallel)
	}
}

func TestStubSessionHandle_Done_ClosesOnCancel(t *testing.T) {
	h := NewStubSessionHandle("s1")
	h.Close()

	select {
	case <-h.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done() did not close after Close()")
	}
}

func TestStubSessionHandle_Result_ReturnsSetValue(t *testing.T) {
	h := NewStubSessionHandle("s1")
	h.SetResult("my-result")

	if got := h.Result(); got != "my-result" {
		t.Errorf("Result = %v, want %q", got, "my-result")
	}
}

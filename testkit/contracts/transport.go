package contracts

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/toolkit"
)

// RunTransportContract runs the Transport compliance suite against
// any Transport implementation produced by the factory.
func RunTransportContract(t *testing.T, factory func() toolkit.Transport) {
	t.Helper()

	t.Run("Serve_BlocksUntilContextCancel", func(t *testing.T) {
		tr := factory()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := tr.Serve(ctx, nil)
		// Must return when context is cancelled — either nil or context error
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Serve returned unexpected error: %v", err)
		}
	})

	t.Run("Shutdown_NoPanic", func(t *testing.T) {
		tr := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Shutdown panicked: %v", r)
				}
			}()
			_ = tr.Shutdown(context.Background())
		}()
	})
}

// RunTriggerContract runs the Trigger compliance suite.
func RunTriggerContract(t *testing.T, factory func() toolkit.Trigger, handle toolkit.SessionHandle) {
	t.Helper()

	t.Run("Start_ReturnsHandle", func(t *testing.T) {
		tr := factory()
		h, err := tr.Start(context.Background(), toolkit.TriggerParams{})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if h == nil {
			t.Fatal("Start returned nil handle")
		}
	})

	t.Run("Start_HandleHasID", func(t *testing.T) {
		tr := factory()
		h, err := tr.Start(context.Background(), toolkit.TriggerParams{})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if h.ID() == "" {
			t.Error("SessionHandle.ID() is empty")
		}
	})

	t.Run("Start_ContextCancellation", func(t *testing.T) {
		tr := factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Should not panic on cancelled context
		_, _ = tr.Start(ctx, toolkit.TriggerParams{})
	})
}

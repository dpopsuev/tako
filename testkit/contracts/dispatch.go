package contracts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/dispatch"
)

// RunDispatcherContract verifies the Dispatch → GetNextStep → SubmitArtifact
// round-trip contract. The MuxDispatcher must route artifacts back to the
// correct Dispatch caller.
func RunDispatcherContract(t *testing.T) {
	t.Helper()

	t.Run("RoundTrip_SingleDispatch", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mux := dispatch.NewMuxDispatcher(ctx)

		var result []byte
		var dispatchErr error
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			result, dispatchErr = mux.Dispatch(ctx, dispatch.Context{
				CaseID:        "test-1",
				Step:          "step-a",
				PromptContent: "hello",
			})
		}()

		// Agent side: pull and submit.
		dc, err := mux.GetNextStep(ctx)
		if err != nil {
			t.Fatalf("GetNextStep: %v", err)
		}
		if dc.CaseID != "test-1" {
			t.Errorf("CaseID = %q, want test-1", dc.CaseID)
		}
		if dc.PromptContent != "hello" {
			t.Errorf("PromptContent = %q, want hello", dc.PromptContent)
		}

		if err := mux.SubmitArtifact(ctx, dc.DispatchID, []byte("response")); err != nil {
			t.Fatalf("SubmitArtifact: %v", err)
		}

		wg.Wait()

		if dispatchErr != nil {
			t.Fatalf("Dispatch: %v", dispatchErr)
		}
		if string(result) != "response" {
			t.Errorf("result = %q, want response", string(result))
		}
	})

	t.Run("RoundTrip_ConcurrentDispatches", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mux := dispatch.NewMuxDispatcher(ctx)
		const n = 5

		results := make([][]byte, n)
		errs := make([]error, n)
		var wg sync.WaitGroup

		// Launch N dispatches.
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = mux.Dispatch(ctx, dispatch.Context{
					CaseID:        "concurrent",
					Step:          "step",
					PromptContent: "prompt",
				})
			}()
		}

		// Agent side: pull and submit N times.
		for range n {
			dc, err := mux.GetNextStep(ctx)
			if err != nil {
				t.Fatalf("GetNextStep: %v", err)
			}
			if err := mux.SubmitArtifact(ctx, dc.DispatchID, []byte("ok")); err != nil {
				t.Fatalf("SubmitArtifact: %v", err)
			}
		}

		wg.Wait()

		for i := range n {
			if errs[i] != nil {
				t.Errorf("dispatch %d: %v", i, errs[i])
			}
			if string(results[i]) != "ok" {
				t.Errorf("dispatch %d result = %q, want ok", i, string(results[i]))
			}
		}
	})

	t.Run("ActiveDispatches_TracksCorrectly", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mux := dispatch.NewMuxDispatcher(ctx)

		if mux.ActiveDispatches() != 0 {
			t.Errorf("initial active = %d, want 0", mux.ActiveDispatches())
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mux.Dispatch(ctx, dispatch.Context{CaseID: "track", Step: "s"})
		}()

		// Wait for dispatch to register.
		time.Sleep(50 * time.Millisecond) //nolint:mnd // test timing

		if mux.ActiveDispatches() != 1 {
			t.Errorf("active = %d, want 1", mux.ActiveDispatches())
		}

		dc, _ := mux.GetNextStep(ctx)
		_ = mux.SubmitArtifact(ctx, dc.DispatchID, []byte("done"))
		wg.Wait()

		if mux.ActiveDispatches() != 0 {
			t.Errorf("final active = %d, want 0", mux.ActiveDispatches())
		}
	})
}

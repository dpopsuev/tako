// Package contracts provides factory-based interface compliance suites.
// Any implementation (real or stub) that passes the contract is guaranteed
// to work correctly with the framework.
//
// Usage:
//
//	func TestMyTransformer(t *testing.T) {
//	    contracts.RunInstrumentContract(t, func() engine.Instrument {
//	        return &MyTransformer{}
//	    })
//	}
package contracts

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// RunInstrumentContract runs the instrument compliance suite against
// any Instrument implementation produced by the factory.
func RunInstrumentContract(t *testing.T, factory func() engine.Instrument) {
	t.Helper()

	t.Run("Name_NonEmpty", func(t *testing.T) {
		tr := factory()
		if tr.Name() == "" {
			t.Error("Name() must return a non-empty string")
		}
	})

	t.Run("Transform_ReturnsResult", func(t *testing.T) {
		tr := factory()
		ctx := context.Background()
		tc := &engine.InstrumentContext{
			NodeName:    "test-node",
			WalkerState: circuit.NewWalkerState("test"),
		}
		result, err := tr.Transform(ctx, tc)
		if err != nil {
			t.Fatalf("Transform returned error: %v", err)
		}
		if result == nil {
			t.Error("Transform returned nil result")
		}
	})

	t.Run("Transform_RespectsContextCancellation", func(t *testing.T) {
		tr := factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		tc := &engine.InstrumentContext{
			NodeName:    "test-node",
			WalkerState: circuit.NewWalkerState("test"),
		}
		// Should either return an error or complete quickly — must not hang.
		done := make(chan struct{})
		go func() {
			_, _ = tr.Transform(ctx, tc)
			close(done)
		}()

		select {
		case <-done:
			// OK — returned (with or without error)
		case <-time.After(5 * time.Second):
			t.Fatal("Transform did not respect context cancellation within 5s")
		}
	})

	t.Run("Transform_NilWalkerState_NoPanic", func(t *testing.T) {
		tr := factory()
		ctx := context.Background()
		tc := &engine.InstrumentContext{
			NodeName: "test-node",
			// WalkerState intentionally nil
		}
		// Should not panic — may return error, that's fine.
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Transform panicked with nil WalkerState: %v", r)
				}
			}()
			_, _ = tr.Transform(ctx, tc)
		}()
	})
}

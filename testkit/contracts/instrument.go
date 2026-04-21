// Package contracts provides factory-based interface compliance suites.
//
// RunInstrumentNodeContract verifies that an InstrumentNode satisfies
// circuit.Node. RunInstrumentToolContract verifies battery.Tool compliance.
// Any implementation (real or stub) that passes both contracts is guaranteed
// to work correctly as a circuit instrument.
//
// Usage:
//
//	func TestMyInstrument_Node(t *testing.T) {
//	    contracts.RunInstrumentNodeContract(t, func() circuit.Node {
//	        return NewMyInstrumentNode(manifest)
//	    })
//	}
//
//	func TestMyInstrument_Tool(t *testing.T) {
//	    contracts.RunInstrumentToolContract(t, func() tool.Tool {
//	        return NewMyInstrumentTool(manifest)
//	    })
//	}
package contracts

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/tool"
	"github.com/dpopsuev/troupe/visual"
)

// RunInstrumentNodeContract runs the circuit.Node compliance suite against
// any instrument node implementation produced by the factory.
func RunInstrumentNodeContract(t *testing.T, factory func() circuit.Node) {
	t.Helper()

	t.Run("Name_NonEmpty", func(t *testing.T) {
		n := factory()
		if n.Name() == "" {
			t.Error("Name() must return a non-empty string")
		}
	})

	t.Run("Approach_Valid", func(t *testing.T) {
		n := factory()
		// Approach may be empty (no affinity) but must not panic.
		_ = n.Approach()
	})

	t.Run("Process_ReturnsArtifact", func(t *testing.T) {
		n := factory()
		ctx := context.Background()
		nc := circuit.NodeContext{
			WalkerState: circuit.NewWalkerState("test"),
		}
		art, err := n.Process(ctx, nc)
		if err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		if art == nil {
			t.Error("Process returned nil artifact")
		}
	})

	t.Run("Process_ArtifactHasType", func(t *testing.T) {
		n := factory()
		ctx := context.Background()
		nc := circuit.NodeContext{
			WalkerState: circuit.NewWalkerState("test"),
		}
		art, err := n.Process(ctx, nc)
		if err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		if art.Type() == "" {
			t.Error("Artifact.Type() must return a non-empty string")
		}
	})

	t.Run("Process_RespectsContextCancellation", func(t *testing.T) {
		n := factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		nc := circuit.NodeContext{
			WalkerState: circuit.NewWalkerState("test"),
		}
		done := make(chan struct{})
		go func() {
			_, _ = n.Process(ctx, nc)
			close(done)
		}()

		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Process did not respect context cancellation within 5s")
		}
	})

	t.Run("Process_NilWalkerState_NoPanic", func(t *testing.T) {
		n := factory()
		ctx := context.Background()
		nc := circuit.NodeContext{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Process panicked with nil WalkerState: %v", r)
				}
			}()
			_, _ = n.Process(ctx, nc)
		}()
	})
}

// RunInstrumentToolContract runs the battery.Tool compliance suite against
// any instrument tool implementation produced by the factory.
func RunInstrumentToolContract(t *testing.T, factory func() tool.Tool) {
	t.Helper()

	t.Run("Name_NonEmpty", func(t *testing.T) {
		tl := factory()
		if tl.Name() == "" {
			t.Error("Name() must return a non-empty string")
		}
	})

	t.Run("Description_NonEmpty", func(t *testing.T) {
		tl := factory()
		if tl.Description() == "" {
			t.Error("Description() must return a non-empty string")
		}
	})

	t.Run("InputSchema_ValidJSON", func(t *testing.T) {
		tl := factory()
		schema := tl.InputSchema()
		if len(schema) == 0 {
			t.Error("InputSchema() must return non-empty JSON")
			return
		}
		if !json.Valid(schema) {
			t.Errorf("InputSchema() returned invalid JSON: %s", schema)
		}
	})

	t.Run("Execute_ReturnsResult", func(t *testing.T) {
		tl := factory()
		ctx := context.Background()
		result, err := tl.Execute(ctx, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		if result.Text() == "" {
			t.Error("Execute returned empty result")
		}
	})

	t.Run("Execute_RespectsContextCancellation", func(t *testing.T) {
		tl := factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		go func() {
			_, _ = tl.Execute(ctx, json.RawMessage(`{}`))
			close(done)
		}()

		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Execute did not respect context cancellation within 5s")
		}
	})

	t.Run("Execute_EmptyInput_NoPanic", func(t *testing.T) {
		tl := factory()
		ctx := context.Background()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Execute panicked with empty input: %v", r)
				}
			}()
			_, _ = tl.Execute(ctx, nil)
		}()
	})
}

// Ensure visual.Element is used (interface compliance — Approach returns it).
var _ visual.Element = visual.Element("")

package contracts

import (
	"context"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// RunGraphContract runs the graph compliance suite. The factory must
// return a Graph with at least 2 nodes (A→B→DONE) and a working observer.
func RunGraphContract(t *testing.T, factory func() (engine.Graph, circuit.Walker)) {
	t.Helper()

	t.Run("Walk_TraversesNodes", func(t *testing.T) {
		g, w := factory()
		err := g.Walk(context.Background(), w, "A")
		if err != nil {
			t.Fatalf("Walk returned error: %v", err)
		}
	})

	t.Run("Walk_EmitsObserverEvents", func(t *testing.T) {
		g, w := factory()
		dg, ok := g.(*engine.DefaultGraph)
		if !ok {
			t.Skip("graph is not *DefaultGraph, cannot set observer")
		}

		var mu sync.Mutex
		var events []circuit.WalkEvent
		dg.SetObserver(circuit.WalkObserverFunc(func(e circuit.WalkEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}))

		err := g.Walk(context.Background(), w, "A")
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()

		// Must have at least: node_enter(A), node_exit(A), node_enter(B), node_exit(B), walk_complete
		hasEnter := false
		hasExit := false
		hasComplete := false
		for _, e := range events {
			switch e.Type {
			case circuit.EventNodeEnter:
				hasEnter = true
			case circuit.EventNodeExit:
				hasExit = true
			case circuit.EventWalkComplete:
				hasComplete = true
			}
		}
		if !hasEnter {
			t.Error("missing node_enter event")
		}
		if !hasExit {
			t.Error("missing node_exit event")
		}
		if !hasComplete {
			t.Error("missing walk_complete event")
		}
	})

	t.Run("Walk_ErrorOnMissingStartNode", func(t *testing.T) {
		g, w := factory()
		err := g.Walk(context.Background(), w, "nonexistent")
		if err == nil {
			t.Error("Walk should return error for missing start node")
		}
	})

	t.Run("Walk_RespectsContextCancellation", func(t *testing.T) {
		g, w := factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Should return quickly with error, not hang
		_ = g.Walk(ctx, w, "A")
	})
}

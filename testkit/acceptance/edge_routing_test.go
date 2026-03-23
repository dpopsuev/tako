package acceptance

// Feature: Edge Routing
//   As a circuit designer
//   I want conditional edges to route walks based on artifact output
//   So that circuits can branch dynamically

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// walkerWithContext creates a ProcessWalker with pre-populated context fields.
func walkerWithContext(id string, ctx map[string]any) circuit.Walker {
	w := circuit.NewProcessWalker(id)
	for k, v := range ctx {
		w.State().Context[k] = v
	}
	return w
}

func TestEdgeRouting_ExpressionEdgeTransitionsOnTrue(t *testing.T) {
	// Scenario: Expression edge evaluates true and transitions
	//   Given the branching circuit with a shortcut at confidence >= 0.8
	//   When the walker context sets confidence = 0.95
	//   Then the walk takes the fast-path (shortcut)

	tc := &engine.TraceCollector{}
	w := walkerWithContext("test", map[string]any{"confidence": 0.95})

	err := runFixture(t, "circuits/branching.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	visitedNodes := nodeNames(tc.EventsOfType(circuit.EventNodeEnter))

	if !contains(visitedNodes, "fast-path") {
		t.Errorf("expected fast-path in visited nodes, got %v", visitedNodes)
	}
	if contains(visitedNodes, "detailed") {
		t.Errorf("detailed should NOT be visited with confidence 0.95, got %v", visitedNodes)
	}
}

func TestEdgeRouting_ExpressionEdgeFallsThrough(t *testing.T) {
	// Scenario: Expression edge evaluates false and falls through
	//   Given the branching circuit with a shortcut at confidence >= 0.8
	//   When the walker context sets confidence = 0.5
	//   Then the walk takes the detailed path

	tc := &engine.TraceCollector{}
	w := walkerWithContext("test", map[string]any{"confidence": 0.5})

	err := runFixture(t, "circuits/branching.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	visitedNodes := nodeNames(tc.EventsOfType(circuit.EventNodeEnter))

	if !contains(visitedNodes, "detailed") {
		t.Errorf("expected detailed path with confidence 0.5, got %v", visitedNodes)
	}
	if contains(visitedNodes, "fast-path") {
		t.Errorf("fast-path should NOT be visited with confidence 0.5, got %v", visitedNodes)
	}
}

func TestEdgeRouting_LoopEdgeIncrementsCounter(t *testing.T) {
	// Scenario: Loop edge increments counter and re-enters node
	//   Given the looping circuit with max_refine_loops = 2
	//   When convergence stays below threshold
	//   Then the refine node is visited 3 times (initial + 2 loops)

	tc := &engine.TraceCollector{}
	w := walkerWithContext("test", map[string]any{"convergence": 0.1}) // never converges

	err := runFixture(t, "circuits/looping.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	enters := tc.EventsOfType(circuit.EventNodeEnter)
	refineCount := 0
	for _, e := range enters {
		if e.Node == "refine" {
			refineCount++
		}
	}

	if refineCount != 3 {
		t.Errorf("refine visited %d times, want 3 (1 initial + 2 loops)", refineCount)
	}
}

func TestEdgeRouting_ShortcutSkipsNodes(t *testing.T) {
	// Scenario: Shortcut edge skips nodes in cascade
	//   Given the branching circuit with a shortcut edge (confidence >= 0.8)
	//   When confidence is high, the shortcut fires
	//   Then classify goes directly to fast-path, skipping detailed

	// The branching circuit already tests this pattern:
	// classify → fast-path (shortcut, confidence >= 0.8)
	// classify → detailed (fallback, confidence < 0.8)
	// This test verifies the shortcut edge IS marked shortcut: true in the fixture.

	def := loadFixture(t, "circuits/branching.yaml")

	// Verify the fixture declares a shortcut edge
	foundShortcut := false
	for _, e := range def.Edges {
		if e.Shortcut {
			foundShortcut = true
			if e.From != "classify" || e.To != "fast-path" {
				t.Errorf("shortcut edge from=%q to=%q, want classify→fast-path", e.From, e.To)
			}
		}
	}
	if !foundShortcut {
		t.Fatal("branching.yaml has no shortcut edge")
	}

	// Walk with high confidence — shortcut should fire
	tc := &engine.TraceCollector{}
	w := walkerWithContext("test", map[string]any{"confidence": 0.95})

	err := runFixture(t, "circuits/branching.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	visitedNodes := nodeNames(tc.EventsOfType(circuit.EventNodeEnter))
	if contains(visitedNodes, "detailed") {
		t.Errorf("shortcut should skip detailed, got %v", visitedNodes)
	}
}

func nodeNames(events []circuit.WalkEvent) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.Node
	}
	return names
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

package assertions

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// AssertTraceContains verifies that at least one TraceEvent exists matching
// the given caseID, event type, and node name.
func AssertTraceContains(t testing.TB, events []engine.TraceEvent, caseID, eventType, node string) {
	t.Helper()

	for _, e := range events {
		if e.CaseID == caseID && e.Event == eventType && e.Node == node {
			return
		}
	}
	t.Errorf("trace does not contain event (case=%q, event=%q, node=%q)", caseID, eventType, node)
}

// AssertPath verifies that the trace events for a given caseID contain
// the expected node sequence (based on node_enter events, in order).
func AssertPath(t testing.TB, events []engine.TraceEvent, caseID string, expectedNodes []string) {
	t.Helper()

	var actualNodes []string
	for _, e := range events {
		if e.CaseID == caseID && e.Event == string(circuit.EventNodeEnter) {
			actualNodes = append(actualNodes, e.Node)
		}
	}

	if len(actualNodes) != len(expectedNodes) {
		t.Errorf("path length mismatch for case %q: got %d nodes %v, want %d nodes %v",
			caseID, len(actualNodes), actualNodes, len(expectedNodes), expectedNodes)
		return
	}

	for i := range expectedNodes {
		if actualNodes[i] != expectedNodes[i] {
			t.Errorf("path mismatch at index %d for case %q: got %q, want %q (full path: %v)",
				i, caseID, actualNodes[i], expectedNodes[i], actualNodes)
			return
		}
	}
}

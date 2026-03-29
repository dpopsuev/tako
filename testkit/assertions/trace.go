package assertions

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// AssertTraceContains verifies that at least one TraceEvent exists matching
// the given caseID, event type, and node name.
func AssertTraceContains(tb testing.TB, events []engine.TraceEvent, caseID, eventType, node string) {
	tb.Helper()

	for i := range events {
		if events[i].CaseID == caseID && events[i].Event == eventType && events[i].Node == node {
			return
		}
	}
	tb.Errorf("trace does not contain event (case=%q, event=%q, node=%q)", caseID, eventType, node)
}

// AssertPath verifies that the trace events for a given caseID contain
// the expected node sequence (based on node_enter events, in order).
func AssertPath(tb testing.TB, events []engine.TraceEvent, caseID string, expectedNodes []string) {
	tb.Helper()

	var actualNodes []string
	for i := range events {
		if events[i].CaseID == caseID && events[i].Event == string(circuit.EventNodeEnter) {
			actualNodes = append(actualNodes, events[i].Node)
		}
	}

	if len(actualNodes) != len(expectedNodes) {
		tb.Errorf("path length mismatch for case %q: got %d nodes %v, want %d nodes %v",
			caseID, len(actualNodes), actualNodes, len(expectedNodes), expectedNodes)
		return
	}

	for i := range expectedNodes {
		if actualNodes[i] != expectedNodes[i] {
			tb.Errorf("path mismatch at index %d for case %q: got %q, want %q (full path: %v)",
				i, caseID, actualNodes[i], expectedNodes[i], actualNodes)
			return
		}
	}
}

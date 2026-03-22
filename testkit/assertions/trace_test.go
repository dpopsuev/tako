package assertions_test

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/assertions"
)

func TestAssertTraceContains_Found(t *testing.T) {
	events := []engine.TraceEvent{
		{CaseID: "C01", Event: "node_enter", Node: "recall"},
		{CaseID: "C01", Event: "node_exit", Node: "recall"},
		{CaseID: "C02", Event: "node_enter", Node: "triage"},
	}

	assertions.AssertTraceContains(t, events, "C01", "node_enter", "recall")
	assertions.AssertTraceContains(t, events, "C02", "node_enter", "triage")
}

func TestAssertPath_Matching(t *testing.T) {
	events := []engine.TraceEvent{
		{CaseID: "C01", Event: string(circuit.EventNodeEnter), Node: "recall"},
		{CaseID: "C01", Event: string(circuit.EventNodeExit), Node: "recall"},
		{CaseID: "C01", Event: string(circuit.EventNodeEnter), Node: "triage"},
		{CaseID: "C01", Event: string(circuit.EventNodeExit), Node: "triage"},
		{CaseID: "C01", Event: string(circuit.EventNodeEnter), Node: "investigate"},
	}

	assertions.AssertPath(t, events, "C01", []string{"recall", "triage", "investigate"})
}

func TestAssertPath_FiltersByCase(t *testing.T) {
	events := []engine.TraceEvent{
		{CaseID: "C01", Event: string(circuit.EventNodeEnter), Node: "A"},
		{CaseID: "C02", Event: string(circuit.EventNodeEnter), Node: "X"},
		{CaseID: "C01", Event: string(circuit.EventNodeEnter), Node: "B"},
		{CaseID: "C02", Event: string(circuit.EventNodeEnter), Node: "Y"},
	}

	assertions.AssertPath(t, events, "C01", []string{"A", "B"})
	assertions.AssertPath(t, events, "C02", []string{"X", "Y"})
}

func TestAssertPath_EmptyPath(t *testing.T) {
	events := []engine.TraceEvent{
		{CaseID: "C01", Event: "node_exit", Node: "A"}, // not node_enter
	}

	assertions.AssertPath(t, events, "C01", nil)
}

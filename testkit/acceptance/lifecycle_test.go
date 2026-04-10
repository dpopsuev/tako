package acceptance

// Feature: Circuit Lifecycle
//   As a framework consumer
//   I want to define circuits in YAML, build graphs, and walk them
//   So that I can orchestrate AI-powered workflows

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestLifecycle_LinearCircuitCompletesStartToDone(t *testing.T) {
	// Scenario: Linear circuit completes start to done
	//   Given a linear circuit YAML (step-a → step-b → done)
	//   When I run the circuit
	//   Then no error is returned
	//   And both nodes are visited in order

	tc := &engine.TraceCollector{}
	err := runFixture(t, "circuits/linear.yaml", nil, engine.WithRunObserver(tc))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	enters := tc.EventsOfType(circuit.EventNodeEnter)
	if len(enters) != 2 {
		t.Fatalf("node_enter events = %d, want 2", len(enters))
	}
	if enters[0].Node != "step-a" {
		t.Errorf("first node = %q, want step-a", enters[0].Node)
	}
	if enters[1].Node != "step-b" {
		t.Errorf("second node = %q, want step-b", enters[1].Node)
	}
}

func TestLifecycle_RunEntryPoint(t *testing.T) {
	// Scenario: engine.Run() orchestrates full YAML→Build→Walk pipeline
	//   Given a linear circuit YAML file path
	//   When I call engine.Run()
	//   Then no error is returned

	err := runFixture(t, "circuits/linear.yaml", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestLifecycle_MissingTransformerReturnsBuildError(t *testing.T) {
	// Scenario: YAML with missing transformer returns build error
	//   Given a circuit YAML referencing handler "nonexistent"
	//   When I build the graph with empty registries
	//   Then BuildGraph returns an error

	def := &circuit.CircuitDef{
		Circuit: "missing-handler",
		Start:   "a",
		Done:    "done",
		Nodes:   []circuit.NodeDef{{Name: "a", Instrument: "transformer", Action: "nonexistent"}},
		Edges:   []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
	}

	_, err := engine.BuildGraph(def, &engine.GraphRegistries{})
	if err == nil {
		t.Fatal("expected error for missing transformer, got nil")
	}
}

func TestLifecycle_MalformedYAMLReturnsParseError(t *testing.T) {
	// Scenario: Malformed YAML returns parse error
	//   Given invalid YAML content
	//   When I call LoadCircuit
	//   Then an error is returned

	_, err := circuit.LoadCircuit([]byte("{{invalid yaml}}"))
	if err == nil {
		t.Fatal("expected parse error for malformed YAML")
	}
}

func TestLifecycle_VarsResolveInExpressions(t *testing.T) {
	// Scenario: Circuit with vars resolves template placeholders
	//   Given the looping circuit with vars.convergence_threshold = 0.7
	//   When the walker context has convergence >= 0.7
	//   Then the walk completes via the converged edge

	w := circuit.NewProcessWalker("test")
	w.State().Context["convergence"] = 0.9 // above threshold

	err := runFixture(t, "circuits/looping.yaml", nil,
		engine.WithWalker(w),
	)
	if err != nil {
		t.Fatalf("Run with convergent input: %v", err)
	}
}

func TestLifecycle_CircuitNotFoundReturnsError(t *testing.T) {
	// Scenario: Non-existent circuit file returns error
	//   Given a path to a file that doesn't exist
	//   When I call engine.Run()
	//   Then an error is returned

	err := engine.Run(context.Background(), "/nonexistent/circuit.yaml", nil)
	if err == nil {
		t.Fatal("expected error for missing circuit file")
	}
}

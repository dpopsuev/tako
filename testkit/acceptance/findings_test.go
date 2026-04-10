package acceptance

// Feature: Findings & Enforcement
//   As a circuit designer
//   I want to veto artifacts with error findings and route based on finding counts
//   So that quality gates enforce correctness

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestFindings_VetoHookZeroesConfidence(t *testing.T) {
	// Scenario: VetoHook zeroes artifact confidence when error finding exists
	//   Given a node with an error finding
	//   And a VetoHook registered as after-hook
	//   When the node produces an artifact
	//   Then the artifact confidence is zeroed

	collector := &engine.InMemoryFindingCollector{}
	_ = collector.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingError,
		NodeName: "analyze",
		Domain:   "security",
		Message:  "critical vulnerability detected",
	})

	vetoHook := engine.NewVetoHook(collector)
	hooks := engine.HookRegistry{}
	hooks.Register(vetoHook)

	// Define a simple circuit inline
	def := &circuit.CircuitDef{
		Circuit: "veto-test",
		Start:   "analyze",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "analyze", Instrument: "transformer", Action: "echo", After: []string{"finding-veto"}},
		},
		Edges: []circuit.EdgeDef{
			{ID: "analyze-done", From: "analyze", To: "_done", When: "true"},
		},
	}

	// Build runner (not raw graph) — hooks are processed by Runner, not Graph
	runner, err := engine.NewRunnerWith(def, &engine.GraphRegistries{
		Transformers: standardTransformers(),
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewRunnerWith: %v", err)
	}

	w := circuit.NewProcessWalker("test")

	err = runner.Walk(context.Background(), w, "analyze")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Verify the artifact was vetoed (confidence = 0)
	// The hookingWalker intercepts ErrFindingVeto and wraps with VetoArtifact
	if w.State().Outputs["analyze"] == nil {
		t.Fatal("expected analyze artifact in walker outputs")
	}
	if w.State().Outputs["analyze"].Confidence() != 0 {
		t.Errorf("artifact confidence = %f, want 0 (veto should zero it)", w.State().Outputs["analyze"].Confidence())
	}
}

func TestFindings_ExpressionEdgeFiringOnFindingCount(t *testing.T) {
	// Scenario: Expression edge fires when finding count threshold is met
	//   Given a circuit with an edge: when FindingCount("warning") >= 2
	//   And 2 warning findings reported
	//   When the edge is evaluated
	//   Then the edge fires and transitions to error handler

	collector := &engine.InMemoryFindingCollector{}
	ctx := context.Background()

	// Report 2 warning findings
	_ = collector.Report(ctx, &circuit.Finding{
		Severity: circuit.FindingWarning,
		NodeName: "validate",
		Domain:   "lint",
		Message:  "style violation",
	})
	_ = collector.Report(ctx, &circuit.Finding{
		Severity: circuit.FindingWarning,
		NodeName: "validate",
		Domain:   "lint",
		Message:  "another style issue",
	})

	// Define circuit with expression edge based on finding count
	def := &circuit.CircuitDef{
		Circuit: "finding-count-test",
		Start:   "validate",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "validate", Instrument: "transformer", Action: "echo"},
			{Name: "error-handler", Instrument: "transformer", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{
				ID:   "validate-error",
				From: "validate",
				To:   "error-handler",
				When: `signals.FindingCount("warning") >= 2`,
			},
			{
				ID:   "validate-ok",
				From: "validate",
				To:   "_done",
				When: "true",
			},
			{
				ID:   "error-done",
				From: "error-handler",
				To:   "_done",
				When: "true",
			},
		},
	}

	g, err := engine.BuildGraph(def, &engine.GraphRegistries{
		Transformers: standardTransformers(),
	})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	w := circuit.NewProcessWalker("test")
	w.State().Context[circuit.FindingCollectorKey] = collector

	tc := &engine.TraceCollector{}
	if dg, ok := g.(*engine.DefaultGraph); ok {
		dg.SetObserver(tc)
	}

	err = g.Walk(context.Background(), w, "validate")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Verify error-handler was visited
	enters := tc.EventsOfType(circuit.EventNodeEnter)
	visitedNodes := make(map[string]bool)
	for _, e := range enters {
		visitedNodes[e.Node] = true
	}

	if !visitedNodes["error-handler"] {
		t.Error("error-handler should be visited when FindingCount >= 2")
	}

	// Verify the expression edge fired (transition to error-handler)
	transitions := tc.EventsOfType(circuit.EventTransition)
	foundErrorTransition := false
	for _, tr := range transitions {
		if tr.Node == "validate" && tr.Edge == "validate-error" {
			foundErrorTransition = true
		}
	}
	if !foundErrorTransition {
		t.Error("expected transition from validate via validate-error edge")
	}
}

package acceptance

// Feature: Sub-Circuit Delegation
//   As a circuit designer
//   I want to delegate work to sub-circuits via instrument: circuit
//   So that I can compose circuits hierarchically

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

func TestDelegation_DelegateNodeWalksSubCircuit(t *testing.T) {
	// Scenario: Delegate node walks a sub-circuit to completion
	//   Given a parent circuit that delegates to a child circuit
	//   When I register the child circuit in GraphRegistries.Circuits
	//   And run the parent circuit
	//   Then the walk completes with status done

	parentDef := loadFixture(t, "circuits/subcircuit.yaml")
	childDef := loadFixture(t, "circuits/child.yaml")

	tc := &engine.TraceCollector{}
	registries := standardRegistries()
	registries.Circuits = map[string]*circuit.CircuitDef{
		"child": childDef,
	}

	g, err := engine.BuildGraph(parentDef, registries)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test-delegate")

	if dg, ok := g.(*engine.DefaultGraph); ok {
		dg.SetObserver(tc)
	}

	err = g.Walk(context.Background(), walker, string(parentDef.Start))
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Verify walk completed (reached done node)
	if walker.State().Status != "done" {
		t.Errorf("walker status = %q, want done", walker.State().Status)
	}

	// Verify delegate events were emitted
	delegateStarts := tc.EventsOfType(circuit.EventDelegateStart)
	if len(delegateStarts) == 0 {
		t.Error("expected at least one delegate_start event, got none")
	}

	delegateEnds := tc.EventsOfType(circuit.EventDelegateEnd)
	if len(delegateEnds) == 0 {
		t.Error("expected at least one delegate_end event, got none")
	}
}

func TestDelegation_DelegateArtifactWrapsChildOutput(t *testing.T) {
	// Scenario: Delegate artifact wraps the child circuit's output
	//   Given a parent circuit that delegates to a child circuit
	//   When I run the parent circuit
	//   Then the delegate node's artifact is a DelegateArtifact
	//   And it contains the child circuit's inner artifacts

	parentDef := loadFixture(t, "circuits/subcircuit.yaml")
	childDef := loadFixture(t, "circuits/child.yaml")

	registries := standardRegistries()
	registries.Circuits = map[string]*circuit.CircuitDef{
		"child": childDef,
	}

	g, err := engine.BuildGraph(parentDef, registries)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test-artifact")

	err = g.Walk(context.Background(), walker, string(parentDef.Start))
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Verify the main node produced an artifact
	mainArtifact, ok := walker.State().Outputs["main"]
	if !ok {
		t.Fatal("main node artifact missing from outputs")
	}

	// Verify it's a DelegateArtifact
	delegateArt, ok := mainArtifact.(*engine.DelegateArtifact)
	if !ok {
		t.Fatalf("main artifact type = %T, want *engine.DelegateArtifact", mainArtifact)
	}

	// Verify delegate artifact contains inner artifacts
	if len(delegateArt.InnerArtifacts) == 0 {
		t.Error("delegate artifact has no inner artifacts")
	}

	// Verify child circuit node count
	if delegateArt.NodeCount == 0 {
		t.Error("delegate artifact NodeCount = 0, expected > 0")
	}

	// Verify no inner error
	if delegateArt.InnerError != nil {
		t.Errorf("delegate artifact InnerError = %v, want nil", delegateArt.InnerError)
	}

	// Verify namespaced inner artifacts in walker state
	// Child circuit has a "process" node
	nsKey := "delegate:main:process"
	if _, ok := walker.State().Outputs[nsKey]; !ok {
		t.Errorf("expected namespaced inner artifact %q, not found in outputs", nsKey)
	}
}

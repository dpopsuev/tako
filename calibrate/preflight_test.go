package calibrate

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

func TestPreflight_ValidCircuit(t *testing.T) {
	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: testCircuitDef(),
	})
	if err != nil {
		t.Fatalf("expected valid circuit to pass preflight, got: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %+v", len(report.Errors), report.Errors)
	}
	// Should have all phases passed: validate, components, build, walk
	want := map[string]bool{"validate": true, "components": true, "build": true, "walk": true}
	for _, p := range report.Passed {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Errorf("missing passed phases: %v (got %v)", want, report.Passed)
	}
	if report.Elapsed <= 0 {
		t.Error("expected positive elapsed duration")
	}
}

func TestPreflight_MissingTransformer(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "broken-circuit",
		Start:   "start",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "start", Instrument: "transformer", Action: "nonexistent-transformer"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "start-done", From: "start", To: "done"},
		},
	}

	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for missing transformer, got nil")
	}
	if report == nil {
		t.Fatal("expected non-nil report even on error")
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected at least one error in report")
	}
	t.Logf("got expected error: %v (phase: %s)", err, report.Errors[0].Phase)
}

func TestPreflight_InvalidStartNode(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "bad-start",
		Start:   "nonexistent",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "start", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "start-done", From: "start", To: "done"},
		},
	}

	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for invalid start node, got nil")
	}
	if report == nil {
		t.Fatal("expected non-nil report even on error")
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected at least one error in report")
	}
	t.Logf("got expected error: %v (phase: %s)", err, report.Errors[0].Phase)
}

func TestPreflight_NilCircuitDef(t *testing.T) {
	report, err := Preflight(context.Background(), &HarnessConfig{})
	if err == nil {
		t.Fatal("expected error for nil CircuitDef, got nil")
	}
	if report == nil {
		t.Fatal("expected non-nil report even on error")
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected error in report for nil CircuitDef")
	}
	if report.Errors[0].Phase != "validate" {
		t.Errorf("expected validate phase error, got %s", report.Errors[0].Phase)
	}
}

func TestPreflight_CompletesQuickly(t *testing.T) {
	start := time.Now()
	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: testCircuitDef(),
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("preflight took %v, expected < 1s", elapsed)
	}
	if report.Elapsed <= 0 || report.Elapsed > 1*time.Second {
		t.Errorf("report.Elapsed = %v, expected positive and < 1s", report.Elapsed)
	}
	t.Logf("preflight completed in %v", elapsed)
}

func TestPreflight_MultiNodeCircuit(t *testing.T) {
	// Circuit with multiple nodes; preflight should still pass because
	// it only validates graph construction and enters the start node.
	def := &circuit.CircuitDef{
		Circuit: "multi-node",
		Start:   "a",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "a", Instrument: "transformer", Action: "passthrough"},
			{Name: "b", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "a-b", From: "a", To: "b"},
			{ID: "b-done", From: "b", To: "done"},
		},
	}

	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: def,
	})
	if err != nil {
		t.Fatalf("expected multi-node circuit to pass preflight, got: %v", err)
	}
	if len(report.Passed) < 4 {
		t.Errorf("expected at least 4 passed phases, got %d: %v", len(report.Passed), report.Passed)
	}
}

func TestPreflight_BrokenEdgeExpression(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "broken-edge",
		Start:   "start",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "start", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "start-done", From: "start", To: "done", When: "invalid_func(!!!"},
		},
	}

	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected error for broken edge expression, got nil")
	}
	if report == nil {
		t.Fatal("expected non-nil report even on error")
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected error in report")
	}
	t.Logf("got expected error: %v (phase: %s)", err, report.Errors[0].Phase)
}

func TestPreflight_PartialReport_OnBuildError(t *testing.T) {
	// A circuit that passes validation but fails on build should have
	// "validate" and "components" in Passed, and "build" in Errors.
	def := &circuit.CircuitDef{
		Circuit: "broken-build",
		Start:   "start",
		Done:    "done",
		Nodes: []circuit.NodeDef{
			{Name: "start", Instrument: "transformer", Action: "nonexistent-transformer"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "start-done", From: "start", To: "done"},
		},
	}

	report, err := Preflight(context.Background(), &HarnessConfig{
		CircuitDef: def,
	})
	if err == nil {
		t.Fatal("expected build error")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// validate should have passed
	hasValidate := false
	for _, p := range report.Passed {
		if p == "validate" {
			hasValidate = true
		}
	}
	if !hasValidate {
		t.Error("expected 'validate' in Passed before build failure")
	}

	// build should be in Errors
	if len(report.Errors) == 0 {
		t.Fatal("expected build error in report")
	}
	if report.Errors[0].Phase != "build" {
		t.Errorf("expected build phase error, got %s", report.Errors[0].Phase)
	}
}

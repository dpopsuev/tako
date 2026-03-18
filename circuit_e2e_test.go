package framework

import (
	"context"
	"os"
	"testing"
)

// loadTestCircuit reads a YAML fixture from testdata/circuits/.
func loadTestCircuit(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/circuits/" + name)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

// walkCircuit is a helper that loads a circuit, builds a graph, and walks
// it with passthrough transformers via BatchWalk.
func walkCircuit(t *testing.T, def *CircuitDef, reg GraphRegistries, input map[string]any) BatchWalkResult {
	t.Helper()
	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:      def,
		Shared:   reg,
		Cases:    []BatchCase{{ID: "test", Context: input}},
		Parallel: 1,
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	return results[0]
}

func passthroughRegistry() GraphRegistries {
	return GraphRegistries{
		Transformers: TransformerRegistry{
			"passthrough": &passthroughTransformer{},
		},
	}
}

// contextTransformer returns the walker's context as the node output.
// This makes context fields (e.g., "confidence") available to edge
// expressions as output.confidence.
type contextTransformer struct{}

func (t *contextTransformer) Name() string { return "context-echo" }
func (t *contextTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	if tc.WalkerState != nil && tc.WalkerState.Context != nil {
		return tc.WalkerState.Context, nil
	}
	return tc.Input, nil
}

// --- E2E Tests ---

func TestCircuitE2E_Linear(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "linear.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := walkCircuit(t, def, passthroughRegistry(), map[string]any{"input": "hello"})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}
	if len(result.Path) < 2 {
		t.Errorf("path too short: %v (want at least [step-a, step-b])", result.Path)
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_Branching_HighConfidence(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "branching.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}

	// High confidence → fast-path shortcut.
	result := walkCircuit(t, def, reg,
		map[string]any{"confidence": 0.9})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}

	pathHas := func(name string) bool {
		for _, p := range result.Path {
			if p == name {
				return true
			}
		}
		return false
	}
	if !pathHas("classify") {
		t.Error("missing 'classify' in path")
	}
	if !pathHas("fast-path") {
		t.Error("expected 'fast-path' for high confidence")
	}
	if pathHas("detailed") {
		t.Error("should NOT take 'detailed' path for high confidence")
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_Branching_LowConfidence(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "branching.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}

	result := walkCircuit(t, def, reg,
		map[string]any{"confidence": 0.3})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}

	pathHas := func(name string) bool {
		for _, p := range result.Path {
			if p == name {
				return true
			}
		}
		return false
	}
	if !pathHas("detailed") {
		t.Error("expected 'detailed' for low confidence")
	}
	if pathHas("fast-path") {
		t.Error("should NOT take 'fast-path' for low confidence")
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_Looping_Converges(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "looping.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}

	// convergence=0.8 > threshold=0.7 → should exit on first refine.
	result := walkCircuit(t, def, reg,
		map[string]any{"convergence": 0.8})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_Looping_Exhausted(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "looping.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}

	// convergence=0.3 < threshold=0.7 → loops until max_refine_loops (2) then exhausted.
	result := walkCircuit(t, def, reg,
		map[string]any{"convergence": 0.3})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}

	// Count refine occurrences in path.
	refineCount := 0
	for _, p := range result.Path {
		if p == "refine" {
			refineCount++
		}
	}
	if refineCount < 2 {
		t.Errorf("refine count = %d, want >= 2 (should loop before exhausting)", refineCount)
	}
	t.Logf("path: %v (refine count: %d)", result.Path, refineCount)
}

func TestCircuitE2E_SubCircuit(t *testing.T) {
	parentData := loadTestCircuit(t, "subcircuit.yaml")
	childData := loadTestCircuit(t, "child.yaml")

	parentDef, err := LoadCircuit(parentData)
	if err != nil {
		t.Fatalf("parse parent: %v", err)
	}
	childDef, err := LoadCircuit(childData)
	if err != nil {
		t.Fatalf("parse child: %v", err)
	}

	reg := passthroughRegistry()
	reg.Circuits = map[string]*CircuitDef{"child": childDef}

	result := walkCircuit(t, parentDef, reg, map[string]any{"input": "test"})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}
	if len(result.Path) == 0 {
		t.Error("empty path — circuit didn't walk")
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_Overlay(t *testing.T) {
	baseData := loadTestCircuit(t, "overlay-base.yaml")
	overlayData := loadTestCircuit(t, "overlay.yaml")

	resolver := func(name string) ([]byte, error) {
		if name == "base" {
			return baseData, nil
		}
		return nil, nil
	}

	def, err := LoadCircuitWithOverlay(overlayData, resolver)
	if err != nil {
		t.Fatalf("overlay merge: %v", err)
	}

	// Verify base fields inherited.
	if def.Start != "intake" {
		t.Errorf("start = %q, want intake (from base)", def.Start)
	}

	result := walkCircuit(t, def, passthroughRegistry(), map[string]any{"input": "test"})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}

	// Verify overlay node is in the path.
	pathHas := func(name string) bool {
		for _, p := range result.Path {
			if p == name {
				return true
			}
		}
		return false
	}
	if !pathHas("enrich") {
		t.Error("missing 'enrich' — overlay node not in path")
	}
	t.Logf("path: %v", result.Path)
}

func TestCircuitE2E_OverlayWithSubCircuit(t *testing.T) {
	baseData := loadTestCircuit(t, "overlay-base.yaml")
	overlayData := loadTestCircuit(t, "overlay-subcircuit.yaml")
	childData := loadTestCircuit(t, "child.yaml")

	baseResolver := func(name string) ([]byte, error) {
		if name == "base" {
			return baseData, nil
		}
		return nil, nil
	}

	def, err := LoadCircuitWithOverlay(overlayData, baseResolver)
	if err != nil {
		t.Fatalf("overlay merge: %v", err)
	}

	childDef, err := LoadCircuit(childData)
	if err != nil {
		t.Fatalf("parse child: %v", err)
	}

	reg := passthroughRegistry()
	reg.Circuits = map[string]*CircuitDef{"child": childDef}

	result := walkCircuit(t, def, reg, map[string]any{"input": "test"})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}
	if len(result.Path) == 0 {
		t.Error("empty path")
	}
	t.Logf("path: %v (overlay + sub-circuit combined)", result.Path)
}

func TestCircuitE2E_Calibration(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "calibration.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Verify calibration contract is declared.
	if def.Calibration == nil {
		t.Fatal("calibration contract not declared")
	}
	if len(def.Calibration.Outputs) != 2 {
		t.Errorf("calibration outputs = %d, want 2", len(def.Calibration.Outputs))
	}

	result := walkCircuit(t, def, passthroughRegistry(), map[string]any{"input": "test"})
	if result.Error != nil {
		t.Fatalf("walk error: %v", result.Error)
	}
	t.Logf("path: %v", result.Path)
}

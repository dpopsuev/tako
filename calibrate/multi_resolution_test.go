package calibrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestBuildResolutionPlans_OneCircuit(t *testing.T) {
	circuits := []CircuitEntry{
		{Name: "A", Circuit: "circuit-a"},
	}
	plans := BuildResolutionPlans(circuits)

	// 1 circuit: 1 unit plan only (no pairwise, no integrated)
	if len(plans) != 1 {
		t.Fatalf("got %d plans, want 1", len(plans))
	}
	if plans[0].Name != "A-unit" {
		t.Errorf("plan[0].Name: got %q, want %q", plans[0].Name, "A-unit")
	}
	if plans[0].Resolution != ResolutionUnit {
		t.Errorf("plan[0].Resolution: got %q, want %q", plans[0].Resolution, ResolutionUnit)
	}
	if len(plans[0].Circuits) != 1 || plans[0].Circuits[0] != "A" {
		t.Errorf("plan[0].Circuits: got %v, want [A]", plans[0].Circuits)
	}
}

func TestBuildResolutionPlans_TwoCircuits(t *testing.T) {
	circuits := []CircuitEntry{
		{Name: "A", Circuit: "circuit-a"},
		{Name: "B", Circuit: "circuit-b"},
	}
	plans := BuildResolutionPlans(circuits)

	// 2 circuits: 2 unit + 1 pairwise + 1 integrated = 4 plans
	if len(plans) != 4 {
		t.Fatalf("got %d plans, want 4", len(plans))
	}

	// Unit plans
	unitCount := 0
	for _, p := range plans {
		if p.Resolution == ResolutionUnit {
			unitCount++
		}
	}
	if unitCount != 2 {
		t.Errorf("unit plans: got %d, want 2", unitCount)
	}

	// Pairwise
	var pairwise *ResolutionPlan
	for i := range plans {
		if plans[i].Resolution == ResolutionPairwise {
			pairwise = &plans[i]
			break
		}
	}
	if pairwise == nil {
		t.Fatal("no pairwise plan found")
	}
	if pairwise.Name != "A-B" {
		t.Errorf("pairwise.Name: got %q, want %q", pairwise.Name, "A-B")
	}
	if len(pairwise.Circuits) != 2 || pairwise.Circuits[0] != "A" || pairwise.Circuits[1] != "B" {
		t.Errorf("pairwise.Circuits: got %v, want [A B]", pairwise.Circuits)
	}

	// Integrated
	var integrated *ResolutionPlan
	for i := range plans {
		if plans[i].Resolution == ResolutionIntegrated {
			integrated = &plans[i]
			break
		}
	}
	if integrated == nil {
		t.Fatal("no integrated plan found")
	}
	if integrated.Name != "integrated" {
		t.Errorf("integrated.Name: got %q, want %q", integrated.Name, "integrated")
	}
	if len(integrated.Circuits) != 2 {
		t.Errorf("integrated.Circuits: got %v, want [A B]", integrated.Circuits)
	}
}

func TestBuildResolutionPlans_ThreeCircuits(t *testing.T) {
	circuits := []CircuitEntry{
		{Name: "A", Circuit: "circuit-a"},
		{Name: "B", Circuit: "circuit-b"},
		{Name: "C", Circuit: "circuit-c"},
	}
	plans := BuildResolutionPlans(circuits)

	// 3 circuits: 3 unit + 3 pairwise (A-B, A-C, B-C) + 1 integrated = 7 plans
	if len(plans) != 7 {
		t.Fatalf("got %d plans, want 7", len(plans))
	}

	unitCount := 0
	pairwiseCount := 0
	integratedCount := 0
	for _, p := range plans {
		switch p.Resolution {
		case ResolutionUnit:
			unitCount++
		case ResolutionPairwise:
			pairwiseCount++
		case ResolutionIntegrated:
			integratedCount++
		}
	}
	if unitCount != 3 {
		t.Errorf("unit plans: got %d, want 3", unitCount)
	}
	if pairwiseCount != 3 {
		t.Errorf("pairwise plans: got %d, want 3", pairwiseCount)
	}
	if integratedCount != 1 {
		t.Errorf("integrated plans: got %d, want 1", integratedCount)
	}
}

func TestWrapForResolution_SetsResolutionMetadata(t *testing.T) {
	base := &circuit.CircuitDef{
		Circuit: "test",
		Nodes:   []circuit.NodeDef{{Name: "a", Action: "a", Instrument: "node"}},
		Edges:   []circuit.EdgeDef{{ID: "a-done", From: "a", To: "done"}},
		Start:   "a",
		Done:    "done",
	}
	plan := ResolutionPlan{
		Name:       "my-unit",
		Resolution: ResolutionUnit,
		Circuits:   []string{"my-circuit"},
	}
	config := DecoratorConfig{}

	wrapped := WrapForResolution(base, &plan, config)

	if wrapped.Vars == nil {
		t.Fatal("Vars is nil")
	}
	if v, ok := wrapped.Vars["_calibration_resolution"]; !ok || v != "unit" {
		t.Errorf("_calibration_resolution: got %v (ok=%v), want %q", v, ok, "unit")
	}
	if v, ok := wrapped.Vars["_calibration_plan"]; !ok || v != "my-unit" {
		t.Errorf("_calibration_plan: got %v (ok=%v), want %q", v, ok, "my-unit")
	}
	if !IsCalibrationWrapped(wrapped) {
		t.Error("WrapForResolution should produce calibration-wrapped circuit")
	}
}

func TestParseResolution_Valid(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Resolution
	}{
		{"unit", ResolutionUnit},
		{"pairwise", ResolutionPairwise},
		{"integrated", ResolutionIntegrated},
	} {
		got, err := ParseResolution(tc.in)
		if err != nil {
			t.Errorf("ParseResolution(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseResolution(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseResolution_Invalid(t *testing.T) {
	_, err := ParseResolution("bad")
	if err == nil {
		t.Error("ParseResolution(bad): expected error")
	}
}

func TestLoadPortStubs_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "code-context.json")
	if err := os.WriteFile(fixture, []byte(`{"files":["a.go","b.go"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stubs := []StubDef{{Port: "alpha.in:code-context", Fixture: fixture}}
	ps, err := LoadPortStubs(stubs)
	if err != nil {
		t.Fatalf("LoadPortStubs: %v", err)
	}
	if !ps.IsPortStubbed("alpha.in:code-context") {
		t.Error("port should be stubbed")
	}
	m, ok := ps.Get("alpha.in:code-context").(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", ps.Get("alpha.in:code-context"))
	}
	if _, ok := m["files"]; !ok {
		t.Error("expected files key in fixture data")
	}
}

func TestLoadPortStubs_MissingFile(t *testing.T) {
	stubs := []StubDef{{Port: "test", Fixture: "/nonexistent/file.json"}}
	_, err := LoadPortStubs(stubs)
	if err == nil {
		t.Error("expected error for missing fixture")
	}
}

func TestLoadPortStubs_Empty(t *testing.T) {
	ps, err := LoadPortStubs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if ps != nil {
		t.Error("expected nil for empty stubs")
	}
}

func TestPortStubs_NilSafe(t *testing.T) {
	var ps PortStubs
	if ps.IsPortStubbed("anything") {
		t.Error("nil PortStubs should not report stubbed")
	}
	if ps.Get("anything") != nil {
		t.Error("nil PortStubs.Get should return nil")
	}
}

func TestGetPortStubs_FromCircuit(t *testing.T) {
	def := &circuit.CircuitDef{
		Vars: map[string]any{
			"_port_stubs": PortStubs{"test.in:x": "hello"},
		},
	}
	ps := GetPortStubs(def)
	if ps == nil {
		t.Fatal("expected port stubs")
	}
	if ps.Get("test.in:x") != "hello" {
		t.Errorf("got %v, want hello", ps.Get("test.in:x"))
	}
}

func TestGetPortStubs_NilDef(t *testing.T) {
	if GetPortStubs(nil) != nil {
		t.Error("nil def should return nil stubs")
	}
}

func TestGetResolution_FromCircuit(t *testing.T) {
	def := &circuit.CircuitDef{
		Vars: map[string]any{
			"_calibration_resolution": "unit",
		},
	}
	if GetResolution(def) != ResolutionUnit {
		t.Errorf("got %q, want unit", GetResolution(def))
	}
}

func TestGetResolution_NotSet(t *testing.T) {
	def := &circuit.CircuitDef{}
	if GetResolution(def) != "" {
		t.Error("expected empty resolution for unwrapped circuit")
	}
}

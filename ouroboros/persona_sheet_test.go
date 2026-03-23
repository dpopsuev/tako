package ouroboros

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

func testProfile() ModelProfile {
	return ModelProfile{
		Model: circuit.ModelIdentity{
			ModelName: "claude-sonnet-4",
			Provider:  "anthropic",
			Version:   "20250514",
		},
		BatteryVersion: SeedBatteryVersion,
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.4,
			DimPersistence:          0.7,
			DimConvergenceThreshold: 0.8,
			DimShortcutAffinity:     0.2,
			DimEvidenceDepth:        0.9,
			DimFailureMode:          0.5,
		},
		ElementMatch: element.ElementWater,
		ElementScores: map[element.Element]float64{
			element.ElementWater: 1.0,
			element.ElementEarth: 0.85,
		},
		SuggestedPersonas: []string{"Seeker", "Sentinel"},
	}
}

func testCircuit() circuit.CircuitDef {
	return circuit.CircuitDef{
		Circuit: "rca-investigation",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
			{Name: "resolve"},
			{Name: "investigate"},
			{Name: "correlate"},
			{Name: "review"},
			{Name: "report"},
		},
	}
}

func TestEmitPersonaSheet_AllStepsHavePersonas(t *testing.T) {
	profile := testProfile()
	circuit := testCircuit()

	sheet, err := EmitPersonaSheet(profile, circuit, testRCAStepDims)
	if err != nil {
		t.Fatalf("EmitPersonaSheet: %v", err)
	}

	if sheet.Model == "" {
		t.Error("model identity is empty")
	}
	if sheet.ElementMatch == "" {
		t.Error("element match is empty")
	}
	if len(sheet.DimensionScores) == 0 {
		t.Error("dimension scores are empty")
	}

	for _, node := range circuit.Nodes {
		if node.Name == "_done" {
			continue
		}
		persona, ok := sheet.SuggestedPersonas[node.Name]
		if !ok {
			t.Errorf("missing persona for step %q", node.Name)
			continue
		}
		if persona == "" {
			t.Errorf("empty persona for step %q", node.Name)
		}
		t.Logf("step %q -> %s", node.Name, persona)
	}
}

func TestEmitPersonaSheet_3StepCircuit_NilStepDims(t *testing.T) {
	profile := testProfile()
	circuit := circuit.CircuitDef{
		Circuit: "ouroboros-probe",
		Nodes: []circuit.NodeDef{
			{Name: "generate"},
			{Name: "subject"},
			{Name: "judge"},
		},
	}

	sheet, err := EmitPersonaSheet(profile, circuit, nil)
	if err != nil {
		t.Fatalf("EmitPersonaSheet: %v", err)
	}

	if len(sheet.SuggestedPersonas) != 3 {
		t.Errorf("personas = %d, want 3", len(sheet.SuggestedPersonas))
	}

	for _, step := range []string{"generate", "subject", "judge"} {
		personaName, ok := sheet.SuggestedPersonas[step]
		if !ok {
			t.Errorf("missing persona for %q", step)
			continue
		}
		if personaName == "" {
			t.Errorf("step %q has empty persona", step)
		}
		t.Logf("step %q -> %s (generalist via ElementMatch)", step, personaName)
	}
}

func TestEmitPersonaSheet_EmptyModel_Error(t *testing.T) {
	profile := ModelProfile{}
	circuit := testCircuit()

	_, err := EmitPersonaSheet(profile, circuit, nil)
	if err == nil {
		t.Fatal("expected error for empty model, got nil")
	}
}

func TestPersonaSheet_MarshalYAML(t *testing.T) {
	profile := testProfile()
	circuit := testCircuit()

	sheet, err := EmitPersonaSheet(profile, circuit, testRCAStepDims)
	if err != nil {
		t.Fatalf("EmitPersonaSheet: %v", err)
	}

	data, err := sheet.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	if len(data) == 0 {
		t.Error("YAML output is empty")
	}

	t.Logf("PersonaSheet YAML:\n%s", string(data))
}

func TestEmitPersonaSheet_WithNonZeroAffinityOnly(t *testing.T) {
	profile := testProfile()
	circuit := testCircuit()

	sheet, err := EmitPersonaSheet(profile, circuit, testRCAStepDims)
	if err != nil {
		t.Fatalf("EmitPersonaSheet: %v", err)
	}

	for step, persona := range sheet.SuggestedPersonas {
		if persona == "" {
			t.Errorf("step %q has empty persona", step)
		}
	}
}

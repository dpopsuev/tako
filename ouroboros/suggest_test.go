package ouroboros

import (
	"math"
	"testing"

	"github.com/dpopsuev/bugle/element"
)

func TestElementMatch_FastModel_MapsToFireOrLightning(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.95,
			DimPersistence:          0.0,
			DimConvergenceThreshold: 0.4,
			DimShortcutAffinity:     0.9,
			DimEvidenceDepth:        0.1,
			DimFailureMode:          0.3,
		},
	}

	match := ElementMatch(profile)
	if match != element.ElementFire && match != element.ElementLightning {
		t.Errorf("expected fire or lightning for fast model, got %s", match)
	}
}

func TestElementMatch_ThoroughModel_MapsToEarthOrWater(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.2,
			DimPersistence:          0.8,
			DimConvergenceThreshold: 0.85,
			DimShortcutAffinity:     0.1,
			DimEvidenceDepth:        0.9,
			DimFailureMode:          0.6,
		},
	}

	match := ElementMatch(profile)
	if match != element.ElementEarth && match != element.ElementWater {
		t.Errorf("expected earth or water for thorough model, got %s", match)
	}
}

func TestElementScores_SumPositive(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.5,
			DimPersistence:          0.5,
			DimConvergenceThreshold: 0.5,
			DimShortcutAffinity:     0.5,
			DimEvidenceDepth:        0.5,
			DimFailureMode:          0.5,
		},
	}

	scores := ElementScores(profile)

	for _, e := range element.AllElements() {
		s, ok := scores[e]
		if !ok {
			t.Errorf("missing score for element %s", e)
			continue
		}
		if s <= 0 || s > 1.0 {
			t.Errorf("score for %s = %f, want (0, 1.0]", e, s)
		}
	}

	var hasMax bool
	for _, s := range scores {
		if math.Abs(s-1.0) < 1e-9 {
			hasMax = true
		}
	}
	if !hasMax {
		t.Error("expected at least one score normalized to 1.0")
	}
}

func TestSuggestPersona_ReturnsTwoSuggestions(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.7,
			DimPersistence:          0.2,
			DimConvergenceThreshold: 0.6,
			DimShortcutAffinity:     0.8,
			DimEvidenceDepth:        0.3,
			DimFailureMode:          0.4,
		},
	}

	personas := SuggestPersona(profile)
	if len(personas) != 2 {
		t.Fatalf("expected 2 persona suggestions, got %d: %v", len(personas), personas)
	}
	for _, p := range personas {
		if p == "" {
			t.Error("persona suggestion should not be empty")
		}
	}
}

// testRCAStepDims provides a sample step-dimension mapping for tests.
// This is the kind of mapping a consumer (e.g., Asterisk/RCA) would supply.
var testRCAStepDims = StepDimensionMap{
	"recall":      {DimSpeed, DimShortcutAffinity},
	"triage":      {DimSpeed, DimConvergenceThreshold},
	"resolve":     {DimEvidenceDepth, DimConvergenceThreshold},
	"investigate": {DimEvidenceDepth, DimPersistence, DimConvergenceThreshold},
	"correlate":   {DimPersistence, DimEvidenceDepth},
	"review":      {DimConvergenceThreshold, DimFailureMode},
	"report":      {DimSpeed, DimEvidenceDepth},
}

func TestDeriveStepAffinity_AllStepsPresent(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.6,
			DimPersistence:          0.4,
			DimConvergenceThreshold: 0.7,
			DimShortcutAffinity:     0.5,
			DimEvidenceDepth:        0.8,
			DimFailureMode:          0.3,
		},
	}

	affinity := DeriveStepAffinity(profile, testRCAStepDims)

	for step := range testRCAStepDims {
		v, ok := affinity[step]
		if !ok {
			t.Errorf("missing affinity for step %s", step)
			continue
		}
		if v < 0 || v > 1.0 {
			t.Errorf("affinity[%s] = %f, want [0, 1.0]", step, v)
		}
	}
}

func TestDeriveStepAffinity_NilMap_ReturnsNil(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{DimSpeed: 0.5},
	}
	if result := DeriveStepAffinity(profile, nil); result != nil {
		t.Errorf("expected nil for nil stepDims, got %v", result)
	}
}

func TestDeriveStepAffinity_FastModel_HighRecall(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.95,
			DimPersistence:          0.1,
			DimConvergenceThreshold: 0.3,
			DimShortcutAffinity:     0.9,
			DimEvidenceDepth:        0.1,
			DimFailureMode:          0.2,
		},
	}

	affinity := DeriveStepAffinity(profile, testRCAStepDims)

	if affinity["recall"] < affinity["investigate"] {
		t.Errorf("fast model should have higher recall (%f) than investigate (%f)",
			affinity["recall"], affinity["investigate"])
	}
}

func TestDeriveStepAffinity_DeepModel_HighInvestigate(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{
			DimSpeed:                0.1,
			DimPersistence:          0.9,
			DimConvergenceThreshold: 0.85,
			DimShortcutAffinity:     0.1,
			DimEvidenceDepth:        0.95,
			DimFailureMode:          0.6,
		},
	}

	affinity := DeriveStepAffinity(profile, testRCAStepDims)

	if affinity["investigate"] < affinity["recall"] {
		t.Errorf("deep model should have higher investigate (%f) than recall (%f)",
			affinity["investigate"], affinity["recall"])
	}
}

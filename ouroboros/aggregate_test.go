package ouroboros

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func approxEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestProfileFromPoleResults_Aggregation(t *testing.T) {
	results := []SeedResult{
		{
			Name:       "probe-a",
			Difficulty: DifficultyEasy,
			Result: PoleResult{
				SelectedPole: "systematic",
				Confidence:   0.9,
				DimensionScores: map[Dimension]float64{
					DimSpeed:         0.3,
					DimEvidenceDepth: 0.9,
				},
			},
		},
		{
			Name:       "probe-b",
			Difficulty: DifficultyEasy,
			Result: PoleResult{
				SelectedPole: "methodical",
				Confidence:   0.8,
				DimensionScores: map[Dimension]float64{
					DimSpeed:         0.5,
					DimEvidenceDepth: 0.7,
					DimPersistence:   0.8,
				},
			},
		},
	}

	model := circuit.ModelIdentity{ModelName: "test-model", Provider: "test"}
	profile := ProfileFromPoleResults(model, results)

	if profile.BatteryVersion != SeedBatteryVersion {
		t.Errorf("battery version = %q, want %q", profile.BatteryVersion, SeedBatteryVersion)
	}
	if len(profile.RawResults) != 2 {
		t.Errorf("raw results = %d, want 2", len(profile.RawResults))
	}

	wantSpeed := (0.3 + 0.5) / 2
	if !approxEqual(profile.Dimensions[DimSpeed], wantSpeed, 0.01) {
		t.Errorf("speed = %v, want %v", profile.Dimensions[DimSpeed], wantSpeed)
	}

	wantEvidence := (0.9 + 0.7) / 2
	if !approxEqual(profile.Dimensions[DimEvidenceDepth], wantEvidence, 0.01) {
		t.Errorf("evidence_depth = %v, want %v", profile.Dimensions[DimEvidenceDepth], wantEvidence)
	}

	wantPersistence := 0.8
	if !approxEqual(profile.Dimensions[DimPersistence], wantPersistence, 0.01) {
		t.Errorf("persistence = %v, want %v", profile.Dimensions[DimPersistence], wantPersistence)
	}
}

func TestProfileFromPoleResults_DifficultyWeighting(t *testing.T) {
	results := []SeedResult{
		{
			Name:       "easy-probe",
			Difficulty: DifficultyEasy,
			Result: PoleResult{
				SelectedPole:    "a",
				Confidence:      0.9,
				DimensionScores: map[Dimension]float64{DimSpeed: 0.0},
			},
		},
		{
			Name:       "hard-probe",
			Difficulty: DifficultyHard,
			Result: PoleResult{
				SelectedPole:    "b",
				Confidence:      0.9,
				DimensionScores: map[Dimension]float64{DimSpeed: 1.0},
			},
		},
	}

	model := circuit.ModelIdentity{ModelName: "test", Provider: "test"}
	profile := ProfileFromPoleResults(model, results)

	// easy weight=1, hard weight=3: weighted avg = (0*1 + 1*3) / (1+3) = 0.75
	want := 0.75
	if !approxEqual(profile.Dimensions[DimSpeed], want, 0.01) {
		t.Errorf("speed = %v, want %v (hard should weigh 3x)", profile.Dimensions[DimSpeed], want)
	}
}

func TestProfileFromPoleResults_ElementMatchWorks(t *testing.T) {
	results := []SeedResult{
		{
			Name: "deep-probe",
			Result: PoleResult{
				SelectedPole: "deep",
				Confidence:   0.9,
				DimensionScores: map[Dimension]float64{
					DimSpeed:                0.2,
					DimPersistence:          1.0,
					DimConvergenceThreshold: 0.85,
					DimShortcutAffinity:     0.1,
					DimEvidenceDepth:        0.8,
					DimFailureMode:          0.5,
				},
			},
		},
	}

	model := circuit.ModelIdentity{ModelName: "deep-thinker", Provider: "test"}
	profile := ProfileFromPoleResults(model, results)

	if profile.ElementMatch == "" {
		t.Fatal("ElementMatch is empty")
	}
	if len(profile.ElementScores) == 0 {
		t.Fatal("ElementScores is empty")
	}
	if len(profile.SuggestedPersonas) == 0 {
		t.Fatal("SuggestedPersonas is empty")
	}

	t.Logf("ElementMatch: %s", profile.ElementMatch)
	t.Logf("SuggestedPersonas: %v", profile.SuggestedPersonas)
}

func TestProfileFromPoleResults_DeriveStepAffinityWorks(t *testing.T) {
	results := []SeedResult{
		{
			Name: "balanced-probe",
			Result: PoleResult{
				SelectedPole: "systematic",
				Confidence:   0.9,
				DimensionScores: map[Dimension]float64{
					DimSpeed:                0.4,
					DimPersistence:          0.6,
					DimConvergenceThreshold: 0.7,
					DimShortcutAffinity:     0.3,
					DimEvidenceDepth:        0.8,
					DimFailureMode:          0.5,
				},
			},
		},
	}

	model := circuit.ModelIdentity{ModelName: "balanced", Provider: "test"}
	profile := ProfileFromPoleResults(model, results)

	stepDims := StepDimensionMap{
		"recall":      {DimSpeed, DimShortcutAffinity},
		"investigate": {DimEvidenceDepth, DimPersistence, DimConvergenceThreshold},
	}
	affinity := DeriveStepAffinity(profile, stepDims)
	if len(affinity) == 0 {
		t.Fatal("DeriveStepAffinity returned empty map")
	}

	for step := range stepDims {
		if _, ok := affinity[step]; !ok {
			t.Errorf("missing step affinity for %q", step)
		}
	}

	if affinity["investigate"] <= 0 {
		t.Error("investigate affinity should be > 0 with evidence_depth=0.8")
	}
}

func TestPoleResultToProbeResult_Fields(t *testing.T) {
	pr := &PoleResult{
		SelectedPole: "systematic",
		Confidence:   0.85,
		DimensionScores: map[Dimension]float64{
			DimSpeed:         0.3,
			DimEvidenceDepth: 0.9,
		},
		Reasoning:       "Shows thorough analysis",
		GoldSignalScore: 0.75,
	}

	result := PoleResultToProbeResult("test-seed", pr, 0, DifficultyMedium)
	if result.ProbeID != "test-seed" {
		t.Errorf("ProbeID = %q, want test-seed", result.ProbeID)
	}
	if result.DimensionScores[DimSpeed] != 0.3 {
		t.Errorf("speed = %v, want 0.3", result.DimensionScores[DimSpeed])
	}
	if result.RawOutput != "Shows thorough analysis" {
		t.Errorf("RawOutput = %q, want reasoning text", result.RawOutput)
	}
	if result.Difficulty != DifficultyMedium {
		t.Errorf("Difficulty = %q, want %q", result.Difficulty, DifficultyMedium)
	}
	if result.GoldSignalScore != 0.75 {
		t.Errorf("GoldSignalScore = %v, want 0.75", result.GoldSignalScore)
	}
}

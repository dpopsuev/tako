package ouroboros

import (
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

const BatteryVersion = "ouroboros-v1"
const SeedBatteryVersion = "ouroboros-seed-v1"

// difficultyWeight maps difficulty tiers to aggregation weights.
// Hard probes count 3x, medium 2x, easy 1x. Unset difficulty defaults to 1.0.
var difficultyWeight = map[string]float64{
	DifficultyEasy:   1.0,
	DifficultyMedium: 2.0,
	DifficultyHard:   3.0,
}

// aggregateDimensions computes a weighted average of dimension scores.
// Probes with a difficulty tier receive proportionally more weight.
func aggregateDimensions(profile *ModelProfile) {
	weightedSums := make(map[Dimension]float64)
	totalWeights := make(map[Dimension]float64)

	for _, result := range profile.RawResults {
		w := difficultyWeight[result.Difficulty]
		if w == 0 {
			w = 1.0
		}
		for dim, score := range result.DimensionScores {
			weightedSums[dim] += score * w
			totalWeights[dim] += w
		}
	}

	for _, dim := range AllDimensions() {
		if totalWeights[dim] > 0 {
			profile.Dimensions[dim] = weightedSums[dim] / totalWeights[dim]
		}
	}
}

// PoleResultToProbeResult converts a judge-produced PoleResult into a
// ProbeResult suitable for dimension aggregation. This bridges the seed
// circuit output into the existing ModelProfile aggregation path.
func PoleResultToProbeResult(seedName string, pr *PoleResult, elapsed time.Duration, difficulty string) ProbeResult {
	return ProbeResult{
		ProbeID:         seedName,
		RawOutput:       pr.Reasoning,
		DimensionScores: pr.DimensionScores,
		Elapsed:         elapsed,
		Difficulty:      difficulty,
		GoldSignalScore: pr.GoldSignalScore,
		TimedOut:        pr.TimedOut,
		HintsUsed:       pr.HintsUsed,
	}
}

// SeedResult pairs a PoleResult with seed metadata for aggregation.
type SeedResult struct {
	Name       string
	Difficulty string
	Result     PoleResult
}

// ProfileFromPoleResults aggregates multiple seed circuit PoleResults into
// a ModelProfile, using difficulty-weighted dimension averaging.
func ProfileFromPoleResults(
	model circuit.ModelIdentity,
	results []SeedResult,
) ModelProfile {
	profile := ModelProfile{
		Model:          model,
		BatteryVersion: SeedBatteryVersion,
		Timestamp:      time.Now(),
		Dimensions:     make(map[Dimension]float64),
		ElementScores:  make(map[element.Element]float64),
	}

	for _, sr := range results {
		profile.RawResults = append(profile.RawResults,
			PoleResultToProbeResult(sr.Name, &sr.Result, 0, sr.Difficulty))
	}

	aggregateDimensions(&profile)

	profile.ElementMatch = ElementMatch(profile)
	profile.ElementScores = ElementScores(profile)
	profile.SuggestedPersonas = SuggestPersona(profile)

	return profile
}

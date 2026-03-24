package ouroboros

import (
	"math"
	"sort"

	"github.com/dpopsuev/origami/agentport"
)

// elementVector returns the canonical trait values for an element,
// normalized to 0.0-1.0 across the six behavioral dimensions.
// Speed and MaxLoops are ordinal-to-continuous mappings;
// ConvergenceThreshold and ShortcutAffinity are already 0-1;
// EvidenceDepth is normalized against the max (10).
// FailureMode is mapped to a resilience score (higher = more resilient).
func elementVector(e agentport.Element) map[Dimension]float64 {
	traits := agentport.DefaultTraits(e)

	speedMap := map[agentport.SpeedClass]float64{
		agentport.SpeedFastest:  1.0,
		agentport.SpeedFast:     0.8,
		agentport.SpeedSteady:   0.5,
		agentport.SpeedPrecise:  0.4,
		agentport.SpeedDeep:     0.2,
		agentport.SpeedHolistic: 0.6,
	}

	failureModeResilience := map[string]float64{
		"burns out (token waste)":          0.4,
		"brittle (wrong path, no recovery)": 0.1,
		"bloat (too many steps)":            0.6,
		"shatters (ambiguity kills it)":     0.2,
		"slow (analysis paralysis)":         0.5,
		"floaty (vague, no evidence)":       0.3,
	}

	return map[Dimension]float64{
		DimSpeed:                speedMap[traits.Speed],
		DimPersistence:          float64(traits.MaxLoops) / 3.0,
		DimConvergenceThreshold: traits.ConvergenceThreshold,
		DimShortcutAffinity:     traits.ShortcutAffinity,
		DimEvidenceDepth:        float64(traits.EvidenceDepth) / 10.0,
		DimFailureMode:          failureModeResilience[traits.FailureMode],
	}
}

// ElementMatch returns the element whose canonical trait vector is closest
// to the profile's measured dimensions (Euclidean distance).
func ElementMatch(profile ModelProfile) agentport.Element {
	scores := ElementScores(profile)

	var bestElement agentport.Element
	bestScore := -1.0
	for _, e := range agentport.AllElements() {
		if scores[e] > bestScore {
			bestScore = scores[e]
			bestElement = e
		}
	}
	return bestElement
}

// ElementScores returns an affinity score (0-1) for each core agentport.
// Higher means the profile is more similar to that element's canonical traits.
// Computed as 1 / (1 + distance) then normalized so the max is 1.0.
func ElementScores(profile ModelProfile) map[agentport.Element]float64 {
	raw := make(map[agentport.Element]float64)
	var maxRaw float64

	for _, e := range agentport.AllElements() {
		vec := elementVector(e)
		dist := euclideanDistance(profile.Dimensions, vec)
		score := 1.0 / (1.0 + dist)
		raw[e] = score
		if score > maxRaw {
			maxRaw = score
		}
	}

	result := make(map[agentport.Element]float64, len(raw))
	for e, score := range raw {
		if maxRaw > 0 {
			result[e] = score / maxRaw
		}
	}
	return result
}

func euclideanDistance(a, b map[Dimension]float64) float64 {
	var sum float64
	for _, dim := range AllDimensions() {
		av := a[dim]
		bv := b[dim]
		d := av - bv
		sum += d * d
	}
	return math.Sqrt(sum)
}

// SuggestPersona returns real persona names suitable for a model profile,
// based on element match. Resolves the top 2 element affinities to the
// closest Thesis persona from the persona/ package. Falls back to element
// name + "-primary" if no persona matches the agentport.
func SuggestPersona(profile ModelProfile) []string {
	scores := ElementScores(profile)

	type scored struct {
		element agentport.Element
		score   float64
	}
	var sorted []scored
	for e, s := range scores {
		sorted = append(sorted, scored{e, s})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	limit := 2
	if len(sorted) < limit {
		limit = len(sorted)
	}

	var suggestions []string
	for i := 0; i < limit; i++ {
		name := personaNameForElement(sorted[i].element)
		suggestions = append(suggestions, name)
	}
	return suggestions
}

// personaNameForElement returns the Thesis persona name whose element matches,
// falling back to element + "-primary" if none match.
func personaNameForElement(e agentport.Element) string {
	for _, p := range agentport.PersonaThesis() {
		if p.Identity.Element == e {
			return p.Identity.PersonaName
		}
	}
	for _, p := range agentport.PersonaAntithesis() {
		if p.Identity.Element == e {
			return p.Identity.PersonaName
		}
	}
	return string(e) + "-primary"
}

// DeriveStepAffinity computes a per-step affinity score from the profile's
// measured dimensions and a consumer-provided step-to-dimension mapping.
// Steps not present in stepDims are omitted from the result.
// Returns nil when stepDims is nil or empty.
func DeriveStepAffinity(profile ModelProfile, stepDims StepDimensionMap) map[string]float64 {
	if len(stepDims) == 0 {
		return nil
	}
	dims := profile.Dimensions
	result := make(map[string]float64, len(stepDims))
	for step, dimList := range stepDims {
		vals := make([]float64, len(dimList))
		for i, d := range dimList {
			vals[i] = dims[d]
		}
		result[step] = avg(vals...)
	}
	return result
}


func avg(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

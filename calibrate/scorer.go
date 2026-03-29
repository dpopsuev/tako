package calibrate

import (
	"fmt"
	"strings"
)

// ScorerFunc computes a single metric value from a case result and ground truth.
// Returns (value, detail, error) where detail is a human-readable explanation
// (e.g. "15/20 correct"). params carries metric-specific configuration from
// the scorecard YAML (e.g. field names to compare).
type ScorerFunc func(caseResult, groundTruth any, params map[string]any) (float64, string, error)

// ScorerRegistry maps scorer names to implementations. Built-in scorers are
// pre-registered; consumers add domain-specific scorers at startup.
type ScorerRegistry map[string]ScorerFunc

// Register adds a scorer. Panics on duplicate.
func (r ScorerRegistry) Register(name string, fn ScorerFunc) {
	if _, exists := r[name]; exists {
		panic(fmt.Sprintf("duplicate scorer registration: %q", name))
	}
	r[name] = fn
}

// Get returns the scorer registered under name, or an error if not found.
func (r ScorerRegistry) Get(name string) (ScorerFunc, error) {
	if r == nil {
		return nil, ErrScorerRegistryIsNil
	}
	fn, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q not registered", ErrScorer, name)
	}
	return fn, nil
}

// DefaultScorerRegistry returns a registry pre-loaded with built-in scorers
// (3 per-case + 10 batch patterns).
func DefaultScorerRegistry() ScorerRegistry {
	reg := make(ScorerRegistry)
	reg["accuracy"] = accuracyScorer
	reg["rate"] = rateScorer
	reg["threshold_check"] = thresholdCheckScorer
	RegisterBatchScorers(reg)
	return reg
}

// accuracyScorer compares two field values for equality.
// params: "predicted" (field name in caseResult), "expected" (field name in groundTruth).
// Both caseResult and groundTruth must be map[string]any. Returns 1.0 on match, 0.0 otherwise.
func accuracyScorer(caseResult, groundTruth any, params map[string]any) (score float64, detail string, err error) {
	predictedField, _ := params["predicted"].(string)
	expectedField, _ := params["expected"].(string)
	if predictedField == "" || expectedField == "" {
		return 0, "", ErrAccuracyScorerParamsMustIncludePredictedAndExpectedF
	}

	resultMap, ok := caseResult.(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("%w: %T", ErrAccuracyScorerCaseResultMustBeMapStringAnyGot, caseResult)
	}
	truthMap, ok := groundTruth.(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("%w: %T", ErrAccuracyScorerGroundTruthMustBeMapStringAnyGot, groundTruth)
	}

	predicted := fmt.Sprintf("%v", resultMap[predictedField])
	expected := fmt.Sprintf("%v", truthMap[expectedField])

	if strings.EqualFold(predicted, expected) {
		return 1.0, fmt.Sprintf("%s=%s (match)", predictedField, predicted), nil
	}
	return 0.0, fmt.Sprintf("%s=%s vs %s=%s", predictedField, predicted, expectedField, expected), nil
}

// rateScorer counts how many items in a list field match a condition.
// params: "field" (list field in caseResult), "match_field" (field in groundTruth list),
// "result_field" and "truth_field" for comparing elements.
// Both caseResult and groundTruth must be map[string]any with list fields.
// Returns matched/total.
func rateScorer(caseResult, groundTruth any, params map[string]any) (score float64, detail string, err error) {
	field, _ := params["field"].(string)
	if field == "" {
		return 0, "", ErrRateScorerParamsMustIncludeField
	}

	truthMap, ok := groundTruth.(map[string]any)
	if !ok {
		return 0, "", ErrRateScorerGroundTruthMustBeMapStringAny
	}

	truthItems, ok := truthMap[field].([]any)
	if !ok {
		return 0, "0/0", nil
	}
	if len(truthItems) == 0 {
		return 1.0, "0/0 (vacuous)", nil
	}

	resultMap, ok := caseResult.(map[string]any)
	if !ok {
		return 0, "", ErrRateScorerCaseResultMustBeMapStringAny
	}

	resultItems, ok := resultMap[field].([]any)
	if !ok {
		return 0, fmt.Sprintf("0/%d", len(truthItems)), nil
	}

	resultSet := make(map[string]bool, len(resultItems))
	for _, item := range resultItems {
		resultSet[fmt.Sprintf("%v", item)] = true
	}

	matched := 0
	for _, item := range truthItems {
		if resultSet[fmt.Sprintf("%v", item)] {
			matched++
		}
	}

	return float64(matched) / float64(len(truthItems)), fmt.Sprintf("%d/%d", matched, len(truthItems)), nil
}

// thresholdCheckScorer checks if a numeric field meets a threshold.
// params: "field" (field name in caseResult), "min" (minimum value, optional),
// "max" (maximum value, optional). Returns 1.0 if within bounds, 0.0 otherwise.
func thresholdCheckScorer(caseResult, _ any, params map[string]any) (score float64, detail string, err error) {
	field, _ := params["field"].(string)
	if field == "" {
		return 0, "", ErrThresholdCheckScorerParamsMustIncludeField
	}

	resultMap, ok := caseResult.(map[string]any)
	if !ok {
		return 0, "", ErrThresholdCheckScorerCaseResultMustBeMapStringAny
	}

	val, err := toFloat64(resultMap[field])
	if err != nil {
		return 0, "", fmt.Errorf("threshold_check scorer: field %q: %w", field, err)
	}

	if minVal, ok := params["min"]; ok {
		minF, err := toFloat64(minVal)
		if err != nil {
			return 0, "", fmt.Errorf("threshold_check scorer: min: %w", err)
		}
		if val < minF {
			return 0.0, fmt.Sprintf("%s=%.2f < min=%.2f", field, val, minF), nil
		}
	}
	if maxVal, ok := params["max"]; ok {
		maxF, err := toFloat64(maxVal)
		if err != nil {
			return 0, "", fmt.Errorf("threshold_check scorer: max: %w", err)
		}
		if val > maxF {
			return 0.0, fmt.Sprintf("%s=%.2f > max=%.2f", field, val, maxF), nil
		}
	}

	return 1.0, fmt.Sprintf("%s=%.2f (pass)", field, val), nil
}

func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("%w: %T to float64", ErrCannotConvert, v)
	}
}

// ScoreCase evaluates a single case against all metrics in the scorecard that
// have a Scorer field. Returns metric ID -> value and metric ID -> detail.
// Metrics without a Scorer field are skipped (backward compatible — consumers
// provide those values via the existing CaseScorer interface).
func (sc *ScoreCard) ScoreCase(caseResult, groundTruth any, reg ScorerRegistry) (metricValues map[string]float64, metricDetails map[string]string, err error) {
	values := make(map[string]float64)
	details := make(map[string]string)
	for i := range sc.MetricDefs {
		def := &sc.MetricDefs[i]
		if def.Scorer == "" {
			continue
		}
		fn, err := reg.Get(def.Scorer)
		if err != nil {
			return nil, nil, fmt.Errorf("metric %s: %w", def.ID, err)
		}
		val, detail, err := fn(caseResult, groundTruth, def.Params)
		if err != nil {
			return nil, nil, fmt.Errorf("metric %s: scorer %q: %w", def.ID, def.Scorer, err)
		}
		values[def.ID] = val
		if detail != "" {
			details[def.ID] = detail
		}
	}
	return values, details, nil
}

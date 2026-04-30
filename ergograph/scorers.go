package ergograph

import (
	"fmt"
	"strings"
)

func DefaultScorerRegistry() ScorerRegistry {
	reg := make(ScorerRegistry)
	reg["accuracy"] = accuracyScorer
	reg["rate"] = rateScorer
	reg["threshold_check"] = thresholdCheckScorer
	return reg
}

func accuracyScorer(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	predictedField := MapStr(AsMap(params), "predicted")
	expectedField := MapStr(AsMap(params), "expected")
	if predictedField == "" || expectedField == "" {
		return 0, "", fmt.Errorf("accuracy scorer: params must include predicted and expected fields")
	}

	resultMap := AsMap(caseResult)
	truthMap := AsMap(groundTruth)
	if resultMap == nil || truthMap == nil {
		return 0, "", fmt.Errorf("accuracy scorer: case result and ground truth must be map[string]any")
	}

	predicted := fmt.Sprintf("%v", resultMap[predictedField])
	expected := fmt.Sprintf("%v", truthMap[expectedField])

	if strings.EqualFold(predicted, expected) {
		return 1.0, fmt.Sprintf("%s=%s (match)", predictedField, predicted), nil
	}
	return 0.0, fmt.Sprintf("%s=%s vs %s=%s", predictedField, predicted, expectedField, expected), nil
}

func rateScorer(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	field := MapStr(AsMap(params), "field")
	if field == "" {
		return 0, "", fmt.Errorf("rate scorer: params must include field")
	}

	truthMap := AsMap(groundTruth)
	if truthMap == nil {
		return 0, "", fmt.Errorf("rate scorer: ground truth must be map[string]any")
	}

	truthItems, ok := truthMap[field].([]any)
	if !ok {
		return 0, "0/0", nil
	}
	if len(truthItems) == 0 {
		return 1.0, "0/0 (vacuous)", nil
	}

	resultMap := AsMap(caseResult)
	if resultMap == nil {
		return 0, "", fmt.Errorf("rate scorer: case result must be map[string]any")
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

func thresholdCheckScorer(caseResult, _ any, params map[string]any) (float64, string, error) {
	field := MapStr(AsMap(params), "field")
	if field == "" {
		return 0, "", fmt.Errorf("threshold_check scorer: params must include field")
	}

	resultMap := AsMap(caseResult)
	if resultMap == nil {
		return 0, "", fmt.Errorf("threshold_check scorer: case result must be map[string]any")
	}

	val := MapFloat(resultMap, field)

	if minVal, ok := params["min"]; ok {
		minF := MapFloat(map[string]any{"v": minVal}, "v")
		if val < minF {
			return 0.0, fmt.Sprintf("%s=%.2f < min=%.2f", field, val, minF), nil
		}
	}
	if maxVal, ok := params["max"]; ok {
		maxF := MapFloat(map[string]any{"v": maxVal}, "v")
		if val > maxF {
			return 0.0, fmt.Sprintf("%s=%.2f > max=%.2f", field, val, maxF), nil
		}
	}

	return 1.0, fmt.Sprintf("%s=%.2f (pass)", field, val), nil
}

package calibrate

import (
	"fmt"
	"math"
	"strings"
)

// RegisterBatchScorers adds all 10 batch-level scorer patterns to the registry.
func RegisterBatchScorers(reg ScorerRegistry) {
	reg.Register("batch_field_match", batchFieldMatch)
	reg.Register("batch_bool_rate", batchBoolRate)
	reg.Register("batch_set_precision", batchSetPrecision)
	reg.Register("batch_set_recall", batchSetRecall)
	reg.Register("batch_set_exclusion", batchSetExclusion)
	reg.Register("batch_keyword_score", batchKeywordScore)
	reg.Register("batch_correlation", batchCorrelation)
	reg.Register("batch_sum_ratio", batchSumRatio)
	reg.Register("batch_field_sum", batchFieldSum)
	reg.Register("batch_group_linkage", batchGroupLinkage)
}

func toBatch(caseResult any) ([]map[string]any, error) {
	batch, ok := caseResult.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("batch scorer: expected []map[string]any, got %T", caseResult)
	}
	return batch, nil
}

// batchFieldMatch compares actual vs expected fields per item.
// params: actual, expected — field names to compare.
//
//	filter — field(s) that must be truthy.
//	fallback_text, fallback_value — if primary match fails,
//	  check if fallback_value appears (case-insensitive) in fallback_text.
func batchFieldMatch(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	actualField := paramString(params, "actual")
	expectedField := paramString(params, "expected")
	if actualField == "" || expectedField == "" {
		return 0, "", fmt.Errorf("batch_field_match: requires 'actual' and 'expected' params")
	}
	filters := paramFilters(params)
	fallbackText := paramString(params, "fallback_text")
	fallbackValue := paramString(params, "fallback_value")

	correct, total := 0, 0
	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		actual, _ := ResolvePath(item, actualField)
		expected, _ := ResolvePath(item, expectedField)
		total++
		if valuesMatch(actual, expected) {
			correct++
		} else if fallbackText != "" && fallbackValue != "" {
			text := ResolveString(item, fallbackText)
			value := ResolveString(item, fallbackValue)
			if text != "" && value != "" && strings.Contains(strings.ToLower(text), strings.ToLower(value)) {
				correct++
			}
		}
	}
	return SafeDivFloat(float64(correct), float64(total)), fmt.Sprintf("%d/%d", correct, total), nil
}

// batchBoolRate counts items matching a boolean filter then checks an actual bool.
// params: filter_field — only items where this field matches filter_value (default true).
//
//	filter_value — expected value for filter_field (default true).
//	actual_field — the bool field to check.
func batchBoolRate(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	filterField := paramString(params, "filter_field")
	actualField := paramString(params, "actual_field")
	if filterField == "" || actualField == "" {
		return 0, "", fmt.Errorf("batch_bool_rate: requires 'filter_field' and 'actual_field' params")
	}
	filterValue := paramBool(params, "filter_value", true)

	matched, expected := 0, 0
	for _, item := range batch {
		if ResolveBool(item, filterField) != filterValue {
			continue
		}
		expected++
		if ResolveBool(item, actualField) {
			matched++
		}
	}
	return SafeDivFloat(float64(matched), float64(expected)), fmt.Sprintf("%d/%d", matched, expected), nil
}

// batchSetPrecision computes set precision: intersection(actual, relevant) / |actual|.
// params: actual_field, relevant_field — string slice fields.
//
//	match_fn — "evidence_overlap" for lenient matching (default: exact).
//	aggregate — "sum" for global numerator/denominator (default: "mean" per-item average).
//	filter — field(s) that must be truthy.
func batchSetPrecision(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	actualField := paramString(params, "actual_field")
	relevantField := paramString(params, "relevant_field")
	if actualField == "" || relevantField == "" {
		return 0, "", fmt.Errorf("batch_set_precision: requires 'actual_field' and 'relevant_field' params")
	}
	filters := paramFilters(params)
	matchFn := paramString(params, "match_fn")
	aggregate := paramString(params, "aggregate")

	sumNumerator, sumDenominator := 0.0, 0.0
	sumPrecision := 0.0
	count := 0

	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		actual := ResolveStringSlice(item, actualField)
		if len(actual) == 0 {
			continue
		}
		relevant := ResolveStringSlice(item, relevantField)
		count++

		var intersect int
		if matchFn == "evidence_overlap" {
			intersect, _ = EvidenceOverlap(actual, relevant)
		} else {
			relSet := make(map[string]bool, len(relevant))
			for _, r := range relevant {
				relSet[r] = true
			}
			for _, a := range actual {
				if relSet[a] {
					intersect++
				}
			}
		}

		if aggregate == "sum" {
			sumNumerator += float64(intersect)
			sumDenominator += float64(len(actual))
		} else {
			sumPrecision += SafeDivFloat(float64(intersect), float64(len(actual)))
		}
	}

	if aggregate == "sum" {
		return SafeDivFloat(sumNumerator, sumDenominator), fmt.Sprintf("%.0f/%.0f", sumNumerator, sumDenominator), nil
	}
	return SafeDivFloat(sumPrecision, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

// batchSetRecall computes set recall: intersection(actual, relevant) / |relevant|.
// Same params as batchSetPrecision.
func batchSetRecall(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	actualField := paramString(params, "actual_field")
	relevantField := paramString(params, "relevant_field")
	if actualField == "" || relevantField == "" {
		return 0, "", fmt.Errorf("batch_set_recall: requires 'actual_field' and 'relevant_field' params")
	}
	filters := paramFilters(params)
	matchFn := paramString(params, "match_fn")
	aggregate := paramString(params, "aggregate")

	sumNumerator, sumDenominator := 0.0, 0.0
	sumRecall := 0.0
	count := 0

	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		count++
		relevant := ResolveStringSlice(item, relevantField)
		if len(relevant) == 0 {
			sumRecall += 1.0
			// vacuously: 0 found / 0 expected, skip regardless of aggregate
			continue
		}
		actual := ResolveStringSlice(item, actualField)

		var intersect int
		if matchFn == "evidence_overlap" {
			intersect, _ = EvidenceOverlap(actual, relevant)
		} else {
			actSet := make(map[string]bool, len(actual))
			for _, a := range actual {
				actSet[a] = true
			}
			for _, r := range relevant {
				if actSet[r] {
					intersect++
				}
			}
		}

		if aggregate == "sum" {
			sumNumerator += float64(intersect)
			sumDenominator += float64(len(relevant))
		} else {
			sumRecall += SafeDivFloat(float64(intersect), float64(len(relevant)))
		}
	}

	if aggregate == "sum" {
		return SafeDivFloat(sumNumerator, sumDenominator), fmt.Sprintf("%.0f/%.0f", sumNumerator, sumDenominator), nil
	}
	return SafeDivFloat(sumRecall, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

// batchSetExclusion computes 1 - (items selecting excluded items / total items).
// params: actual_field — string slice field.
//
//	excluded_field — string slice field in groundTruth (batch-level context).
//	filter — field(s) that must be truthy.
func batchSetExclusion(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	actualField := paramString(params, "actual_field")
	excludedField := paramString(params, "excluded_field")
	if actualField == "" || excludedField == "" {
		return 0, "", fmt.Errorf("batch_set_exclusion: requires 'actual_field' and 'excluded_field' params")
	}
	filters := paramFilters(params)

	gtMap, _ := groundTruth.(map[string]any)
	var excludedSet map[string]bool
	if gtMap != nil {
		excluded := toStringSlice(gtMap[excludedField])
		excludedSet = make(map[string]bool, len(excluded))
		for _, e := range excluded {
			excludedSet[e] = true
		}
	}

	total, selected := 0, 0
	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		actual := ResolveStringSlice(item, actualField)
		if len(actual) == 0 {
			continue
		}
		total++
		for _, a := range actual {
			if excludedSet[a] {
				selected++
				break
			}
		}
	}

	val := 1.0 - SafeDivFloat(float64(selected), float64(total))
	return val, fmt.Sprintf("%d items with selections, %d selected excluded", total, selected), nil
}

// batchKeywordScore scores keyword matches in a text field.
// params: text_field, keywords_field — per-item fields.
//
//	threshold_field — per-item int for min(matched/threshold, 1.0) scoring (M14 mode).
//	hit_threshold — float fraction; items with matched/total >= this count as hits (M14b mode).
//	aggregate — "hit_rate" counts items meeting hit_threshold / eligible (default: "mean").
//	filter — field(s) that must be truthy.
func batchKeywordScore(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	textField := paramString(params, "text_field")
	keywordsField := paramString(params, "keywords_field")
	if textField == "" || keywordsField == "" {
		return 0, "", fmt.Errorf("batch_keyword_score: requires 'text_field' and 'keywords_field' params")
	}
	filters := paramFilters(params)
	thresholdField := paramString(params, "threshold_field")
	hitThreshold, hasHitThreshold := paramFloat(params, "hit_threshold")
	aggregate := paramString(params, "aggregate")

	sumScore := 0.0
	hits, count := 0, 0

	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		text := ResolveString(item, textField)
		keywords := ResolveStringSlice(item, keywordsField)
		if len(keywords) == 0 {
			continue
		}
		count++

		matched := KeywordMatch(text, keywords)

		if aggregate == "hit_rate" && hasHitThreshold {
			if float64(matched) >= float64(len(keywords))*hitThreshold {
				hits++
			}
		} else if thresholdField != "" {
			threshold, ok := ResolveFloat(item, thresholdField)
			if !ok || threshold == 0 {
				threshold = float64(len(keywords))
			}
			sumScore += math.Min(float64(matched)/threshold, 1.0)
		} else {
			sumScore += SafeDivFloat(float64(matched), float64(len(keywords)))
		}
	}

	if aggregate == "hit_rate" {
		return SafeDivFloat(float64(hits), float64(count)), fmt.Sprintf("%d/%d", hits, count), nil
	}
	return SafeDivFloat(sumScore, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

// batchCorrelation computes Pearson correlation between two numeric fields.
// params: x_field, y_field — numeric field names.
//
//	filter — field(s) that must be truthy.
func batchCorrelation(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	xField := paramString(params, "x_field")
	yField := paramString(params, "y_field")
	if xField == "" || yField == "" {
		return 0, "", fmt.Errorf("batch_correlation: requires 'x_field' and 'y_field' params")
	}
	filters := paramFilters(params)

	var xs, ys []float64
	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		x, xOK := ResolveFloat(item, xField)
		y, yOK := ResolveFloat(item, yField)
		if !xOK || !yOK {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}

	corr := PearsonCorrelation(xs, ys)
	return corr, fmt.Sprintf("r=%.2f (n=%d)", corr, len(xs)), nil
}

// batchSumRatio computes sum(numerator_field) / sum(denominator_field).
// params: numerator_field, denominator_field — integer field names.
//
//	zero_both — value when both sums are 0 (default 1.0).
//	zero_denom — value when denominator is 0 but numerator > 0.
func batchSumRatio(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	numField := paramString(params, "numerator_field")
	denomField := paramString(params, "denominator_field")
	if numField == "" || denomField == "" {
		return 0, "", fmt.Errorf("batch_sum_ratio: requires 'numerator_field' and 'denominator_field' params")
	}

	zeroBoth := 1.0
	if v, ok := paramFloat(params, "zero_both"); ok {
		zeroBoth = v
	}

	var sumNum, sumDenom float64
	for _, item := range batch {
		n, _ := ResolveFloat(item, numField)
		d, _ := ResolveFloat(item, denomField)
		sumNum += n
		sumDenom += d
	}

	var val float64
	if sumDenom == 0 && sumNum == 0 {
		val = zeroBoth
	} else if sumDenom == 0 {
		if v, ok := paramFloat(params, "zero_denom"); ok {
			val = v
		} else {
			val = sumNum + 1
		}
	} else {
		val = sumNum / sumDenom
	}
	return val, fmt.Sprintf("actual=%.0f expected=%.0f", sumNum, sumDenom), nil
}

// batchFieldSum sums a numeric field across items. Falls back to an
// estimated value if the primary field is zero.
// params: field — primary numeric field.
//
//	fallback_field — field to use for estimation when primary is 0.
//	fallback_multiplier — multiplier for fallback (e.g. 1000 for tokens-per-step).
func batchFieldSum(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	field := paramString(params, "field")
	if field == "" {
		return 0, "", fmt.Errorf("batch_field_sum: requires 'field' param")
	}
	fallbackField := paramString(params, "fallback_field")
	fallbackMult, hasMult := paramFloat(params, "fallback_multiplier")

	realSum := 0.0
	hasReal := false
	fallbackSum := 0.0

	for _, item := range batch {
		v, _ := ResolveFloat(item, field)
		if v > 0 {
			realSum += v
			hasReal = true
		}
		if fallbackField != "" {
			fv, _ := ResolveFloat(item, fallbackField)
			fallbackSum += fv
		}
	}

	if hasReal {
		return realSum, fmt.Sprintf("%.0f (measured)", realSum), nil
	}
	if hasMult {
		estimated := fallbackSum * fallbackMult
		return estimated, fmt.Sprintf("~%.0f (%.0f * %.0f, estimated)", estimated, fallbackSum, fallbackMult), nil
	}
	return 0, "no data", nil
}

// batchGroupLinkage groups items by a key and checks intra-group consistency.
// All items sharing a group key should have the same value in the value_field.
// Returns correct_links / expected_links.
// params: group_field — field to group by (e.g. rca_id).
//
//	value_field — field that should be consistent within group (e.g. actual_rca_id).
//	filter — field(s) that must be truthy.
func batchGroupLinkage(caseResult, groundTruth any, params map[string]any) (float64, string, error) {
	batch, err := toBatch(caseResult)
	if err != nil {
		return 0, "", err
	}
	groupField := paramString(params, "group_field")
	valueField := paramString(params, "value_field")
	if groupField == "" || valueField == "" {
		return 0, "", fmt.Errorf("batch_group_linkage: requires 'group_field' and 'value_field' params")
	}
	filters := paramFilters(params)

	groups := make(map[string][]any)
	for _, item := range batch {
		if !passesFilters(item, filters) {
			continue
		}
		key := ResolveString(item, groupField)
		if key == "" {
			continue
		}
		val, _ := ResolvePath(item, valueField)
		groups[key] = append(groups[key], val)
	}

	correctLinks, expectedLinks := 0, 0
	for _, vals := range groups {
		if len(vals) < 2 {
			continue
		}
		firstStr := fmt.Sprintf("%v", vals[0])
		for i := 1; i < len(vals); i++ {
			expectedLinks++
			v := fmt.Sprintf("%v", vals[i])
			if v != "" && v != "0" && v != "<nil>" && v == firstStr {
				correctLinks++
			}
		}
	}
	return SafeDivFloat(float64(correctLinks), float64(expectedLinks)), fmt.Sprintf("%d/%d", correctLinks, expectedLinks), nil
}

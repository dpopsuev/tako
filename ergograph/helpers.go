package ergograph

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

func MapStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func MapFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func MapBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func MapStrSlice(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, elem := range v {
			if s, ok := elem.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func AsMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func KeywordMatch(text string, keywords []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

func EvidenceOverlap(actual, expected []string) (found, total int) {
	total = len(expected)
	if total == 0 {
		return 0, 0
	}
	for _, exp := range expected {
		expNorm := filepath.Base(exp)
		matched := false
		for _, act := range actual {
			if strings.Contains(act, expNorm) || strings.Contains(exp, act) || act == exp {
				matched = true
				break
			}
		}
		if !matched {
			expParts := strings.SplitN(exp, ":", 3)
			if len(expParts) >= 2 {
				for _, act := range actual {
					if strings.HasPrefix(act, expParts[0]+":") && strings.Contains(act, expParts[1]) {
						matched = true
						break
					}
				}
			}
		}
		if matched {
			found++
		}
	}
	return found, total
}

func PearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}
	mx, my := Mean(x), Mean(y)
	var num, dx2, dy2 float64
	for i := range x {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		allOne := true
		for _, v := range y {
			if v != 1.0 {
				allOne = false
				break
			}
		}
		if allOne && len(y) > 0 {
			return 1.0
		}
		return 0
	}
	return num / denom
}

func ValuesMatch(a, b any) bool {
	aSlice := toStringSlice(a)
	bSlice := toStringSlice(b)
	if aSlice != nil && bSlice != nil {
		return pathsEqual(aSlice, bSlice)
	}
	return strings.EqualFold(fmt.Sprintf("%v", a), fmt.Sprintf("%v", b))
}

func toStringSlice(v any) []string {
	switch sv := v.(type) {
	case []string:
		return sv
	case []any:
		out := make([]string, len(sv))
		for i, item := range sv {
			out[i] = fmt.Sprintf("%v", item)
		}
		return out
	default:
		return nil
	}
}

func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

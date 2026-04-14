package calibrate

import "strings"

// ResolvePath navigates a nested map using dot-separated keys.
// "alpha.defect_type" walks m["alpha"].(map[string]any)["defect_type"].
// A single-segment path is a direct key lookup with no splitting overhead.
func ResolvePath(m map[string]any, path string) (any, bool) {
	if !strings.Contains(path, ".") {
		v, ok := m[path]
		return v, ok
	}
	parts := strings.Split(path, ".")
	var current any = m
	for _, part := range parts {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = cm[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// ResolveString returns a string value at path, or "" if absent/wrong type.
func ResolveString(m map[string]any, path string) string {
	v, ok := ResolvePath(m, path)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// ResolveBool returns a bool value at path, or false if absent.
func ResolveBool(m map[string]any, path string) bool {
	v, ok := ResolvePath(m, path)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// ResolveFloat returns a float64 at path and whether it was found.
func ResolveFloat(m map[string]any, path string) (float64, bool) {
	v, ok := ResolvePath(m, path)
	if !ok {
		return 0, false
	}
	f, err := toFloat64(v)
	if err != nil {
		return 0, false
	}
	return f, true
}

// ResolveStringSlice returns a []string at path. Handles both []string and
// []any (from YAML unmarshalling).
func ResolveStringSlice(m map[string]any, path string) []string {
	v, ok := ResolvePath(m, path)
	if !ok {
		return nil
	}
	return toStringSlice(v)
}

// isTruthy returns whether a value is non-zero/non-empty/non-nil.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	case []any:
		return len(val) > 0
	case []string:
		return len(val) > 0
	default:
		return true
	}
}

// paramString extracts a string value from scorer params.
func paramString(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return v
}

// paramFloat extracts a float64 from params, accepting int and float types.
func paramFloat(params map[string]any, key string) (float64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	f, err := toFloat64(v)
	if err != nil {
		return 0, false
	}
	return f, true
}

// paramBool extracts a bool from params with a default value.
func paramBool(params map[string]any, key string, defaultVal bool) bool {
	v, ok := params[key].(bool)
	if !ok {
		return defaultVal
	}
	return v
}

// paramFilters returns filter field names from a "filter" param.
// Accepts a single string or a []any of strings.
func paramFilters(params map[string]any) []string {
	v, ok := params["filter"]
	if !ok {
		return nil
	}
	switch fv := v.(type) {
	case string:
		return []string{fv}
	case []any:
		out := make([]string, 0, len(fv))
		for _, item := range fv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return fv
	default:
		return nil
	}
}

// passesFilters checks that all filter fields in the item are truthy.
func passesFilters(item map[string]any, filters []string) bool {
	for _, f := range filters {
		v, ok := ResolvePath(item, f)
		if !ok || !isTruthy(v) {
			return false
		}
	}
	return true
}

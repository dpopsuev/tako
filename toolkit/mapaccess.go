package toolkit

// Typed map[string]any accessors. Safe: return zero values on missing/mistyped keys.

// MapStr extracts a string from a map.
func MapStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// MapFloat extracts a float64 from a map, converting from int/int64 if needed.
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

// MapBool extracts a bool from a map.
func MapBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// MapInt64 extracts an int64 from a map, converting from float64/int if needed.
func MapInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

// MapStrSlice extracts a []string from a map, handling both []string and []any.
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

// MapMap extracts a nested map[string]any.
func MapMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

// MapSlice extracts a []any from a map.
func MapSlice(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

// AsMap safely casts any to map[string]any.
func AsMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

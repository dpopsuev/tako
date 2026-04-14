package calibrate

import "testing"

func TestResolvePath_Simple(t *testing.T) {
	m := map[string]any{"name": "alice"}
	v, ok := ResolvePath(m, "name")
	if !ok || v != "alice" {
		t.Errorf("got %v/%v, want alice/true", v, ok)
	}
}

func TestResolvePath_Nested(t *testing.T) {
	m := map[string]any{
		"alpha": map[string]any{
			"defect_type": "pb001",
		},
	}
	v, ok := ResolvePath(m, "alpha.defect_type")
	if !ok || v != "pb001" {
		t.Errorf("got %v/%v, want pb001/true", v, ok)
	}
}

func TestResolvePath_Missing(t *testing.T) {
	m := map[string]any{"a": "b"}
	_, ok := ResolvePath(m, "missing")
	if ok {
		t.Error("should return false for missing key")
	}
}

func TestResolvePath_NestedMissing(t *testing.T) {
	m := map[string]any{"a": "not a map"}
	_, ok := ResolvePath(m, "a.b")
	if ok {
		t.Error("should return false when intermediate is not a map")
	}
}

func TestResolveString(t *testing.T) {
	m := map[string]any{"name": "alice", "count": 42}
	if s := ResolveString(m, "name"); s != "alice" {
		t.Errorf("got %q, want alice", s)
	}
	if s := ResolveString(m, "count"); s != "" {
		t.Errorf("got %q, want empty (not a string)", s)
	}
	if s := ResolveString(m, "missing"); s != "" {
		t.Errorf("got %q, want empty (missing)", s)
	}
}

func TestResolveBool(t *testing.T) {
	m := map[string]any{"flag": true, "other": "text"}
	if !ResolveBool(m, "flag") {
		t.Error("should return true")
	}
	if ResolveBool(m, "other") {
		t.Error("non-bool should return false")
	}
}

func TestResolveFloat(t *testing.T) {
	m := map[string]any{"score": 0.85, "count": 42, "text": "abc"}
	f, ok := ResolveFloat(m, "score")
	if !ok || f != 0.85 {
		t.Errorf("got %v/%v, want 0.85/true", f, ok)
	}
	f, ok = ResolveFloat(m, "count")
	if !ok || f != 42.0 {
		t.Errorf("got %v/%v, want 42/true", f, ok)
	}
	_, ok = ResolveFloat(m, "text")
	if ok {
		t.Error("string should not resolve as float")
	}
}

func TestResolveStringSlice(t *testing.T) {
	m := map[string]any{
		"repos":   []string{"a", "b"},
		"mixed":   []any{"x", "y"},
		"notlist": "single",
	}
	if s := ResolveStringSlice(m, "repos"); len(s) != 2 || s[0] != "a" {
		t.Errorf("got %v, want [a b]", s)
	}
	if s := ResolveStringSlice(m, "mixed"); len(s) != 2 || s[0] != "x" {
		t.Errorf("got %v, want [x y]", s)
	}
	if s := ResolveStringSlice(m, "notlist"); s != nil {
		t.Errorf("got %v, want nil", s)
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		v    any
		want bool
	}{
		{nil, false},
		{"", false},
		{"abc", true},
		{false, false},
		{true, true},
		{0, false},
		{42, true},
		{0.0, false},
		{0.5, true},
		{[]any{}, false},
		{[]any{"a"}, true},
		{[]string{}, false},
		{[]string{"a"}, true},
	}
	for _, tt := range tests {
		if got := isTruthy(tt.v); got != tt.want {
			t.Errorf("isTruthy(%v) = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func TestParamFilters_String(t *testing.T) {
	filters := paramFilters(map[string]any{"filter": "rca_id"})
	if len(filters) != 1 || filters[0] != "rca_id" {
		t.Errorf("got %v, want [rca_id]", filters)
	}
}

func TestParamFilters_List(t *testing.T) {
	filters := paramFilters(map[string]any{"filter": []any{"a", "b"}})
	if len(filters) != 2 || filters[0] != "a" || filters[1] != "b" {
		t.Errorf("got %v, want [a b]", filters)
	}
}

func TestPassesFilters(t *testing.T) {
	item := map[string]any{"rca_id": "R1", "has_data": true, "empty": ""}
	if !passesFilters(item, []string{"rca_id", "has_data"}) {
		t.Error("should pass with truthy fields")
	}
	if passesFilters(item, []string{"rca_id", "empty"}) {
		t.Error("should fail with empty string field")
	}
	if passesFilters(item, []string{"missing"}) {
		t.Error("should fail with missing field")
	}
}

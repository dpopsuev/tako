package toolkit

import (
	"testing"
)

func TestMapStr(t *testing.T) {
	t.Parallel()
	m := map[string]any{"name": "alice", "count": 42}

	if got := MapStr(m, "name"); got != "alice" {
		t.Errorf("MapStr(name) = %q, want %q", got, "alice")
	}
	if got := MapStr(m, "count"); got != "" {
		t.Errorf("MapStr(count) = %q, want empty (wrong type)", got)
	}
	if got := MapStr(m, "missing"); got != "" {
		t.Errorf("MapStr(missing) = %q, want empty", got)
	}
	if got := MapStr(nil, "any"); got != "" {
		t.Errorf("MapStr(nil map) = %q, want empty", got)
	}
}

func TestMapFloat(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"f64":    3.14,
		"int":    42,
		"int64":  int64(99),
		"str":    "nope",
		"bool":   true,
	}

	if got := MapFloat(m, "f64"); got != 3.14 {
		t.Errorf("MapFloat(f64) = %v, want 3.14", got)
	}
	if got := MapFloat(m, "int"); got != 42 {
		t.Errorf("MapFloat(int) = %v, want 42", got)
	}
	if got := MapFloat(m, "int64"); got != 99 {
		t.Errorf("MapFloat(int64) = %v, want 99", got)
	}
	if got := MapFloat(m, "str"); got != 0 {
		t.Errorf("MapFloat(str) = %v, want 0", got)
	}
	if got := MapFloat(m, "missing"); got != 0 {
		t.Errorf("MapFloat(missing) = %v, want 0", got)
	}
}

func TestMapBool(t *testing.T) {
	t.Parallel()
	m := map[string]any{"on": true, "off": false, "str": "true"}

	if got := MapBool(m, "on"); got != true {
		t.Errorf("MapBool(on) = %v, want true", got)
	}
	if got := MapBool(m, "off"); got != false {
		t.Errorf("MapBool(off) = %v, want false", got)
	}
	if got := MapBool(m, "str"); got != false {
		t.Errorf("MapBool(str) = %v, want false (wrong type)", got)
	}
	if got := MapBool(m, "missing"); got != false {
		t.Errorf("MapBool(missing) = %v, want false", got)
	}
}

func TestMapInt64(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"f64":   float64(7),
		"int64": int64(123),
		"int":   456,
		"str":   "nope",
	}

	if got := MapInt64(m, "f64"); got != 7 {
		t.Errorf("MapInt64(f64) = %v, want 7", got)
	}
	if got := MapInt64(m, "int64"); got != 123 {
		t.Errorf("MapInt64(int64) = %v, want 123", got)
	}
	if got := MapInt64(m, "int"); got != 456 {
		t.Errorf("MapInt64(int) = %v, want 456", got)
	}
	if got := MapInt64(m, "str"); got != 0 {
		t.Errorf("MapInt64(str) = %v, want 0", got)
	}
	if got := MapInt64(m, "missing"); got != 0 {
		t.Errorf("MapInt64(missing) = %v, want 0", got)
	}
}

func TestMapStrSlice(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"typed":   []string{"a", "b"},
		"untyped": []any{"x", "y", 42},
		"wrong":   "not-a-slice",
	}

	got := MapStrSlice(m, "typed")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("MapStrSlice(typed) = %v, want [a b]", got)
	}

	got = MapStrSlice(m, "untyped")
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("MapStrSlice(untyped) = %v, want [x y] (non-strings skipped)", got)
	}

	if got := MapStrSlice(m, "wrong"); got != nil {
		t.Errorf("MapStrSlice(wrong) = %v, want nil", got)
	}
	if got := MapStrSlice(m, "missing"); got != nil {
		t.Errorf("MapStrSlice(missing) = %v, want nil", got)
	}
}

func TestMapMap(t *testing.T) {
	t.Parallel()
	inner := map[string]any{"key": "val"}
	m := map[string]any{"nested": inner, "flat": "nope"}

	if got := MapMap(m, "nested"); got == nil || got["key"] != "val" {
		t.Errorf("MapMap(nested) = %v, want %v", got, inner)
	}
	if got := MapMap(m, "flat"); got != nil {
		t.Errorf("MapMap(flat) = %v, want nil (wrong type)", got)
	}
	if got := MapMap(m, "missing"); got != nil {
		t.Errorf("MapMap(missing) = %v, want nil", got)
	}
}

func TestMapSlice(t *testing.T) {
	t.Parallel()
	items := []any{1, "two", true}
	m := map[string]any{"items": items, "str": "nope"}

	got := MapSlice(m, "items")
	if len(got) != 3 {
		t.Fatalf("MapSlice(items) len = %d, want 3", len(got))
	}
	if got := MapSlice(m, "str"); got != nil {
		t.Errorf("MapSlice(str) = %v, want nil", got)
	}
	if got := MapSlice(m, "missing"); got != nil {
		t.Errorf("MapSlice(missing) = %v, want nil", got)
	}
}

func TestAsMap(t *testing.T) {
	t.Parallel()
	m := map[string]any{"k": "v"}

	if got := AsMap(m); got == nil || got["k"] != "v" {
		t.Errorf("AsMap(map) = %v, want %v", got, m)
	}
	if got := AsMap("string"); got != nil {
		t.Errorf("AsMap(string) = %v, want nil", got)
	}
	if got := AsMap(nil); got != nil {
		t.Errorf("AsMap(nil) = %v, want nil", got)
	}
}

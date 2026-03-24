package toolkit

import (
	"strings"
	"testing"
)

func TestPluralizeCount_Singular(t *testing.T) {
	t.Parallel()
	got := PluralizeCount(1, "failure", "failures")
	if got != "failure" {
		t.Errorf("PluralizeCount(1) = %q, want failure", got)
	}
}

func TestPluralizeCount_Plural(t *testing.T) {
	t.Parallel()
	got := PluralizeCount(0, "item", "items")
	if got != "items" {
		t.Errorf("PluralizeCount(0) = %q, want items", got)
	}
	got = PluralizeCount(5, "item", "items")
	if got != "items" {
		t.Errorf("PluralizeCount(5) = %q, want items", got)
	}
}

func TestSortedKeys(t *testing.T) {
	t.Parallel()
	m := map[string]int{"charlie": 3, "alpha": 1, "bravo": 2}
	keys := SortedKeys(m)
	want := []string{"alpha", "bravo", "charlie"}
	if len(keys) != len(want) {
		t.Fatalf("len = %d, want %d", len(keys), len(want))
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, want[i])
		}
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	t.Parallel()
	keys := SortedKeys[int](nil)
	if len(keys) != 0 {
		t.Errorf("nil map should return empty slice, got %v", keys)
	}
}

func TestGroupByKey(t *testing.T) {
	t.Parallel()
	items := []string{"apple", "avocado", "banana", "blueberry", "cherry"}
	groups := GroupByKey(items, func(s string) string {
		return string(s[0])
	})
	if len(groups["a"]) != 2 {
		t.Errorf("group 'a' len = %d, want 2", len(groups["a"]))
	}
	if len(groups["b"]) != 2 {
		t.Errorf("group 'b' len = %d, want 2", len(groups["b"]))
	}
	if len(groups["c"]) != 1 {
		t.Errorf("group 'c' len = %d, want 1", len(groups["c"]))
	}
}

func TestGroupByKey_Empty(t *testing.T) {
	t.Parallel()
	groups := GroupByKey[string](nil, func(s string) string { return s })
	if len(groups) != 0 {
		t.Errorf("empty input should return empty map, got %v", groups)
	}
}

func TestFormatDistribution_NoLabelFn(t *testing.T) {
	t.Parallel()
	counts := map[string]int{"bug": 5, "env": 2, "auto": 3}
	got := FormatDistribution(counts, nil)
	if !strings.HasPrefix(got, "bug (5)") {
		t.Errorf("expected 'bug (5)' first (highest count), got %q", got)
	}
	if !strings.Contains(got, "auto (3)") {
		t.Errorf("expected 'auto (3)' in output, got %q", got)
	}
	if !strings.Contains(got, "env (2)") {
		t.Errorf("expected 'env (2)' in output, got %q", got)
	}
}

func TestFormatDistribution_WithLabelFn(t *testing.T) {
	t.Parallel()
	counts := map[string]int{"pb001": 3}
	got := FormatDistribution(counts, func(key string) string {
		return "Product Bug [" + key + "]"
	})
	if got != "Product Bug [pb001] (3)" {
		t.Errorf("got %q, want 'Product Bug [pb001] (3)'", got)
	}
}

func TestFormatDistribution_Empty(t *testing.T) {
	t.Parallel()
	got := FormatDistribution(nil, nil)
	if got != "" {
		t.Errorf("empty counts should return empty string, got %q", got)
	}
}

func TestFormatDistribution_TiedCounts(t *testing.T) {
	t.Parallel()
	counts := map[string]int{"a": 2, "b": 2}
	got := FormatDistribution(counts, nil)
	if !strings.Contains(got, "a (2)") || !strings.Contains(got, "b (2)") {
		t.Errorf("expected both items, got %q", got)
	}
}

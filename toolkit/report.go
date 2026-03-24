package toolkit

import (
	"fmt"
	"sort"
	"strings"
)

// PluralizeCount returns singular when n == 1, plural otherwise.
func PluralizeCount(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// SortedKeys returns the keys of a string-keyed map in sorted order.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GroupByKey groups items by a string key extracted via keyFn.
func GroupByKey[T any](items []T, keyFn func(T) string) map[string][]T {
	groups := make(map[string][]T)
	for _, item := range items {
		key := keyFn(item)
		groups[key] = append(groups[key], item)
	}
	return groups
}

// FormatDistribution renders a map of counts as a sorted "key (N), key (N)" string.
// Counts are sorted descending. Labels are passed through labelFn if non-nil.
func FormatDistribution(counts map[string]int, labelFn func(string) string) string {
	type kv struct {
		Key   string
		Count int
	}
	sorted := make([]kv, 0, len(counts))
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })

	parts := make([]string, len(sorted))
	for i, item := range sorted {
		label := item.Key
		if labelFn != nil {
			label = labelFn(item.Key)
		}
		parts[i] = fmt.Sprintf("%s (%d)", label, item.Count)
	}
	return strings.Join(parts, ", ")
}

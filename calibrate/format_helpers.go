package calibrate

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FmtTokens formats a token count with K/M suffix for readability.
func FmtTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000.0)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

// FmtDuration formats a duration as "Xm Ys" or "Ys".
func FmtDuration(d time.Duration) string {
	s := int(d.Seconds())
	if s >= 60 {
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}

// Truncate shortens s to maxLen characters, appending "..." if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// BoolMark returns the check mark for true and the cross mark for false.
func BoolMark(v bool) string {
	if v {
		return "\u2713"
	}
	return "\u2717"
}

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

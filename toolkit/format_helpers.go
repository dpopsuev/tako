package toolkit

import (
	"fmt"
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

// BoolMark returns "✓" for true and "✗" for false.
func BoolMark(v bool) string {
	if v {
		return "✓"
	}
	return "✗"
}

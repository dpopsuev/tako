package probes

import (
	"strings"

	"github.com/dpopsuev/origami/ouroboros"
)

// SummarizeInput is a synthetic PR diff combining three distinct change types:
// a feature addition, a refactoring, and a bug fix. The agent must identify
// and categorize each change separately.
const SummarizeInput = `=== Pull Request #1847: "User dashboard improvements" ===
Files changed: 6, Additions: 142, Deletions: 38

--- a/internal/dashboard/handler.go
+++ b/internal/dashboard/handler.go
@@ -15,8 +15,12 @@
 func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
-    data := h.service.FetchAll(r.Context())
-    json.NewEncoder(w).Encode(data)
+    data, err := h.service.FetchAll(r.Context())
+    if err != nil {
+        http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
+        return
+    }
+    json.NewEncoder(w).Encode(data)
 }

--- a/internal/dashboard/service.go
+++ b/internal/dashboard/service.go
@@ -22,6 +22,24 @@
+// FetchUserMetrics returns aggregated metrics for the authenticated user.
+func (s *Service) FetchUserMetrics(ctx context.Context, userID string) (*UserMetrics, error) {
+    metrics, err := s.repo.GetMetrics(ctx, userID)
+    if err != nil {
+        return nil, fmt.Errorf("fetch metrics for user %s: %w", userID, err)
+    }
+    return &UserMetrics{
+        TotalTests:   metrics.Total,
+        PassRate:     float64(metrics.Passed) / float64(metrics.Total),
+        AvgDuration:  metrics.AvgDuration,
+        LastRunAt:    metrics.LastRun,
+    }, nil
+}

--- a/internal/dashboard/service.go
+++ b/internal/dashboard/service.go
@@ -8,10 +8,8 @@
-func (s *Service) FetchAll(ctx context.Context) interface{} {
-    result, _ := s.repo.GetAll(ctx)
-    return result
-}
+func (s *Service) FetchAll(ctx context.Context) ([]DashboardItem, error) {
+    return s.repo.GetAll(ctx)
+}

--- a/internal/dashboard/cache.go
+++ b/internal/dashboard/cache.go
@@ -31,7 +31,7 @@
 func (c *Cache) Get(key string) (interface{}, bool) {
-    c.mu.Lock()
-    defer c.mu.Unlock()
+    c.mu.RLock()
+    defer c.mu.RUnlock()
     entry, ok := c.items[key]

=== Task ===
Summarize this PR. For each distinct change, state:
1. What changed (one sentence)
2. Category: feature / refactor / bugfix / performance
3. Risk level: low / medium / high`

// BuildSummarizePrompt returns the prompt text using the given stimulus.
func BuildSummarizePrompt(s ProbeStimulus) string {
	return s.Input
}

// SummarizePrompt returns the prompt text using the default stimulus.
func SummarizePrompt() string {
	return BuildSummarizePrompt(DefaultStimuli()["summarize"])
}

// ScoreSummarize maps summarization output to behavioral dimension scores.
// Prefers structured output (CHANGES/OVERALL_RISK/REVIEW_FOCUS fields)
// with keyword fallback for unstructured responses.
func ScoreSummarize(raw string) map[ouroboros.Dimension]float64 {
	lower := strings.ToLower(raw)
	parsed := ParseStructured(raw)

	changesFound := 0
	if parsed.ListLen("CHANGES") > 0 {
		changesFound = parsed.ListLen("CHANGES")
		if changesFound > 4 {
			changesFound = 4
		}
	} else {
		changeSignals := []struct{ keywords []string }{
			{[]string{"fetchusermetrics", "user metrics", "new method", "new function", "feature"}},
			{[]string{"fetchall", "signature", "return error", "refactor"}},
			{[]string{"error handling", "geterror", "http.error", "bug", "bugfix", "fix"}},
			{[]string{"rlock", "read lock", "mutex", "performance", "cache"}},
		}
		for _, cs := range changeSignals {
			for _, kw := range cs.keywords {
				if strings.Contains(lower, kw) {
					changesFound++
					break
				}
			}
		}
	}

	hasCategories := parsed.HasField("OVERALL_RISK") ||
		containsAny(lower, "feature", "refactor", "bugfix", "bug fix", "performance")
	hasRisk := parsed.HasField("OVERALL_RISK") ||
		containsAny(lower, "risk", "low", "medium", "high")
	hasReviewFocus := parsed.HasField("REVIEW_FOCUS")

	evidenceDepth := float64(changesFound) * 0.2
	if hasCategories {
		evidenceDepth += 0.1
	}
	if hasRisk {
		evidenceDepth += 0.1
	}

	lines := strings.Split(raw, "\n")
	nonEmpty := countNonEmpty(lines)
	var failureMode float64
	if nonEmpty > 40 {
		failureMode = 0.2
	} else if nonEmpty > 25 {
		failureMode = 0.4
	} else if nonEmpty >= 8 {
		failureMode = 0.7
	} else {
		failureMode = 0.3
	}
	if hasReviewFocus {
		failureMode += 0.1
	}

	return map[ouroboros.Dimension]float64{
		ouroboros.DimEvidenceDepth: clamp(evidenceDepth),
		ouroboros.DimFailureMode:   clamp(failureMode),
	}
}

package framework

import (
	"context"
	"testing"
)

func TestFindingRouter_AliasesWork(t *testing.T) {
	router := NewFindingRouter(nil, FindingHandlers{})
	ctx := context.Background()

	_ = router.Report(ctx, Finding{Severity: FindingInfo, Message: "a"})
	_ = router.Report(ctx, Finding{Severity: FindingWarning, Message: "b"})

	var _ FindingCollector = router // compile-time check

	findings := router.Findings()
	if len(findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2", len(findings))
	}
}

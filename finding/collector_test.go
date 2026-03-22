package finding

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/core"
)

func TestInMemoryFindingCollector_Report(t *testing.T) {
	c := &InMemoryFindingCollector{}
	ctx := context.Background()

	f1 := core.Finding{Severity: core.FindingInfo, Domain: "lint", Source: "linter", Message: "style issue"}
	f2 := core.Finding{Severity: core.FindingWarning, Domain: "test", Source: "tester", Message: "flaky test"}

	if err := c.Report(ctx, f1); err != nil {
		t.Fatalf("Report f1: %v", err)
	}
	if err := c.Report(ctx, f2); err != nil {
		t.Fatalf("Report f2: %v", err)
	}

	findings := c.Findings()
	if len(findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2", len(findings))
	}
	if findings[0].Severity != core.FindingInfo {
		t.Errorf("findings[0].Severity = %q, want %q", findings[0].Severity, core.FindingInfo)
	}
	if findings[1].Severity != core.FindingWarning {
		t.Errorf("findings[1].Severity = %q, want %q", findings[1].Severity, core.FindingWarning)
	}
}

func TestInMemoryFindingCollector_TimestampDefault(t *testing.T) {
	c := &InMemoryFindingCollector{}
	before := time.Now().UTC()
	if err := c.Report(context.Background(), core.Finding{Severity: core.FindingInfo}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()

	f := c.Findings()[0]
	if f.Timestamp.Before(before) || f.Timestamp.After(after) {
		t.Errorf("Timestamp %v not in [%v, %v]", f.Timestamp, before, after)
	}
}

func TestInMemoryFindingCollector_FindingsReturnsCopy(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), core.Finding{Severity: core.FindingInfo, Message: "original"})

	findings := c.Findings()
	findings[0].Message = "mutated"

	if c.Findings()[0].Message != "original" {
		t.Error("Findings() did not return a copy; mutation leaked")
	}
}

func TestInMemoryFindingCollector_ConcurrentWrites(t *testing.T) {
	c := &InMemoryFindingCollector{}
	ctx := context.Background()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = c.Report(ctx, core.Finding{Severity: core.FindingInfo, Message: "concurrent"})
		}(i)
	}
	wg.Wait()

	if got := len(c.Findings()); got != n {
		t.Errorf("len(Findings) = %d, want %d", got, n)
	}
}

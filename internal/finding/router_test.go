package finding

import (
	"context"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestFindingRouter_DefaultRouting(t *testing.T) {
	var got []RouteTarget
	var mu sync.Mutex
	record := func(target RouteTarget) func(circuit.Finding) {
		return func(_ circuit.Finding) {
			mu.Lock()
			got = append(got, target)
			mu.Unlock()
		}
	}

	router := NewFindingRouter(nil, FindingHandlers{
		Manager: record(TargetManager),
		Broker:  record(TargetBroker),
		Log:     record(TargetLog),
	})

	ctx := context.Background()
	_ = router.Report(ctx, circuit.Finding{Severity: circuit.FindingInfo, Domain: "lint"})
	_ = router.Report(ctx, circuit.Finding{Severity: circuit.FindingWarning, Domain: "test"})
	_ = router.Report(ctx, circuit.Finding{Severity: circuit.FindingError, Domain: "security"})

	if len(got) != 3 {
		t.Fatalf("dispatched %d, want 3", len(got))
	}
	if got[0] != TargetLog {
		t.Errorf("info routed to %q, want %q", got[0], TargetLog)
	}
	if got[1] != TargetManager {
		t.Errorf("warning routed to %q, want %q", got[1], TargetManager)
	}
	if got[2] != TargetBroker {
		t.Errorf("error routed to %q, want %q", got[2], TargetBroker)
	}
}

func TestFindingRouter_ExactDomainMatch(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{{Severity: circuit.FindingWarning, Domain: "test.unit", Target: TargetBroker}},
		FindingHandlers{
			Broker:  func(_ circuit.Finding) { target = TargetBroker },
			Manager: func(_ circuit.Finding) { target = TargetManager },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingWarning, Domain: "test.unit"})
	if target != TargetBroker {
		t.Errorf("exact domain routed to %q, want %q", target, TargetBroker)
	}
}

func TestFindingRouter_GlobDomain(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{{Severity: circuit.FindingWarning, Domain: "test.*", Target: TargetBroker}},
		FindingHandlers{
			Broker:  func(_ circuit.Finding) { target = TargetBroker },
			Manager: func(_ circuit.Finding) { target = TargetManager },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingWarning, Domain: "test.integration"})
	if target != TargetBroker {
		t.Errorf("glob domain routed to %q, want %q", target, TargetBroker)
	}
}

func TestFindingRouter_GlobNoMatch_FallsToDefault(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{{Severity: circuit.FindingWarning, Domain: "test.*", Target: TargetBroker}},
		FindingHandlers{
			Broker:  func(_ circuit.Finding) { target = TargetBroker },
			Manager: func(_ circuit.Finding) { target = TargetManager },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingWarning, Domain: "lint.style"})
	if target != TargetManager {
		t.Errorf("unmatched glob routed to %q, want default %q", target, TargetManager)
	}
}

func TestFindingRouter_FirstMatchWins(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{
			{Severity: circuit.FindingError, Domain: "security.*", Target: TargetBroker},
			{Severity: circuit.FindingError, Domain: "security.*", Target: TargetLog},
		},
		FindingHandlers{
			Broker: func(_ circuit.Finding) { target = TargetBroker },
			Log:    func(_ circuit.Finding) { target = TargetLog },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingError, Domain: "security.auth"})
	if target != TargetBroker {
		t.Errorf("first-match routed to %q, want %q", target, TargetBroker)
	}
}

func TestFindingRouter_SeverityOnlyRule(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{{Severity: circuit.FindingInfo, Target: TargetManager}},
		FindingHandlers{
			Manager: func(_ circuit.Finding) { target = TargetManager },
			Log:     func(_ circuit.Finding) { target = TargetLog },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingInfo, Domain: "anything"})
	if target != TargetManager {
		t.Errorf("severity-only rule routed to %q, want %q", target, TargetManager)
	}
}

func TestFindingRouter_DomainOnlyRule(t *testing.T) {
	var target RouteTarget
	router := NewFindingRouter(
		[]RouteRule{{Domain: "security.*", Target: TargetBroker}},
		FindingHandlers{
			Broker:  func(_ circuit.Finding) { target = TargetBroker },
			Manager: func(_ circuit.Finding) { target = TargetManager },
		},
	)

	_ = router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingWarning, Domain: "security.auth"})
	if target != TargetBroker {
		t.Errorf("domain-only rule routed to %q, want %q", target, TargetBroker)
	}
}

func TestFindingRouter_ImplementsFindingCollector(t *testing.T) {
	router := NewFindingRouter(nil, FindingHandlers{})
	ctx := context.Background()

	_ = router.Report(ctx, circuit.Finding{Severity: circuit.FindingInfo, Message: "a"})
	_ = router.Report(ctx, circuit.Finding{Severity: circuit.FindingWarning, Message: "b"})

	var _ circuit.FindingCollector = router // compile-time check

	findings := router.Findings()
	if len(findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2", len(findings))
	}
}

func TestFindingRouter_NilHandlers(t *testing.T) {
	router := NewFindingRouter(nil, FindingHandlers{})
	err := router.Report(context.Background(), circuit.Finding{Severity: circuit.FindingError})
	if err != nil {
		t.Fatalf("Report with nil handlers: %v", err)
	}
	if len(router.Findings()) != 1 {
		t.Error("finding not collected despite nil handler")
	}
}

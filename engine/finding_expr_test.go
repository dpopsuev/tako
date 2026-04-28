package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

func TestSignalExprHelpers_HasFinding(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingWarning, Domain: "test"})

	h := SignalExprHelpers{Collector: c}

	if !h.HasFinding("warning") {
		t.Error("HasFinding('warning') = false, want true")
	}
	if !h.HasFinding("info") {
		t.Error("HasFinding('info') = false, want true (warning >= info)")
	}
	if h.HasFinding("error") {
		t.Error("HasFinding('error') = true, want false (warning < error)")
	}
}

func TestSignalExprHelpers_HasFinding_NilCollector(t *testing.T) {
	h := SignalExprHelpers{}
	if h.HasFinding("error") {
		t.Error("HasFinding with nil collector should return false")
	}
}

func TestSignalExprHelpers_FindingCount(t *testing.T) {
	c := &InMemoryFindingCollector{}
	ctx := context.Background()
	_ = c.Report(ctx, &circuit.Finding{Severity: circuit.FindingInfo})
	_ = c.Report(ctx, &circuit.Finding{Severity: circuit.FindingWarning})
	_ = c.Report(ctx, &circuit.Finding{Severity: circuit.FindingError})

	h := SignalExprHelpers{Collector: c}

	if got := h.FindingCount("info"); got != 3 {
		t.Errorf("FindingCount('info') = %d, want 3", got)
	}
	if got := h.FindingCount("warning"); got != 2 {
		t.Errorf("FindingCount('warning') = %d, want 2", got)
	}
	if got := h.FindingCount("error"); got != 1 {
		t.Errorf("FindingCount('error') = %d, want 1", got)
	}
}

func TestSignalExprHelpers_FindingDomain(t *testing.T) {
	c := &InMemoryFindingCollector{}
	ctx := context.Background()
	_ = c.Report(ctx, &circuit.Finding{Severity: circuit.FindingInfo, Domain: "test.unit"})
	_ = c.Report(ctx, &circuit.Finding{Severity: circuit.FindingWarning, Domain: "security.auth"})

	h := SignalExprHelpers{Collector: c}

	if !h.FindingDomain("test.*") {
		t.Error("FindingDomain('test.*') = false, want true")
	}
	if !h.FindingDomain("security.*") {
		t.Error("FindingDomain('security.*') = false, want true")
	}
	if h.FindingDomain("lint.*") {
		t.Error("FindingDomain('lint.*') = true, want false")
	}
}

func TestSignalExprHelpers_FindingDomain_NilCollector(t *testing.T) {
	h := SignalExprHelpers{}
	if h.FindingDomain("test.*") {
		t.Error("FindingDomain with nil collector should return false")
	}
}

func TestExpressionEdge_WithFindingCondition(t *testing.T) {
	edge, err := CompileExpressionEdge(&circuit.EdgeDef{
		ID:   "veto-edge",
		From: "nodeA",
		To:   "error-handler",
		When: `signals.HasFinding("error")`,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	collector := &InMemoryFindingCollector{}
	state := circuit.NewWalkerState("test")
	state.Context[circuit.FindingCollectorKey] = collector
	artifact := &stubArtifact{typ: "test", confidence: 0.8, raw: map[string]any{}}

	// No findings: edge should not fire
	if tr := edge.Evaluate(artifact, state); tr != nil {
		t.Error("edge fired with no findings")
	}

	// Add error finding: edge should fire
	_ = collector.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingError, Domain: "security"})
	tr := edge.Evaluate(artifact, state)
	if tr == nil {
		t.Fatal("edge did not fire with FindingError")
	}
	if tr.NextNode != "error-handler" {
		t.Errorf("NextNode = %q, want %q", tr.NextNode, "error-handler")
	}
}

func TestExpressionEdge_FindingCount(t *testing.T) {
	edge, err := CompileExpressionEdge(&circuit.EdgeDef{
		ID:   "multi-finding",
		From: "nodeA",
		To:   "error-handler",
		When: `signals.FindingCount("warning") >= 2`,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	collector := &InMemoryFindingCollector{}
	state := circuit.NewWalkerState("test")
	state.Context[circuit.FindingCollectorKey] = collector
	artifact := &stubArtifact{typ: "test", confidence: 0.8, raw: map[string]any{}}

	_ = collector.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingWarning})
	if tr := edge.Evaluate(artifact, state); tr != nil {
		t.Error("edge fired with only 1 warning")
	}

	_ = collector.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingWarning})
	if tr := edge.Evaluate(artifact, state); tr == nil {
		t.Error("edge did not fire with 2 warnings")
	}
}

func TestExpressionEdge_FindingDomain(t *testing.T) {
	edge, err := CompileExpressionEdge(&circuit.EdgeDef{
		ID:   "domain-edge",
		From: "nodeA",
		To:   "security-handler",
		When: `signals.FindingDomain("security.*")`,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	collector := &InMemoryFindingCollector{}
	state := circuit.NewWalkerState("test")
	state.Context[circuit.FindingCollectorKey] = collector
	artifact := &stubArtifact{typ: "test", confidence: 0.8, raw: map[string]any{}}

	_ = collector.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingInfo, Domain: "test.unit"})
	if tr := edge.Evaluate(artifact, state); tr != nil {
		t.Error("edge fired for non-security domain")
	}

	_ = collector.Report(context.Background(), &circuit.Finding{Severity: circuit.FindingWarning, Domain: "security.auth"})
	if tr := edge.Evaluate(artifact, state); tr == nil {
		t.Error("edge did not fire for security domain")
	}
}

func TestExpressionEdge_NoCollector_GracefulNoop(t *testing.T) {
	edge, err := CompileExpressionEdge(&circuit.EdgeDef{
		ID:   "noop-edge",
		From: "nodeA",
		To:   "error-handler",
		When: `signals.HasFinding("error")`,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	state := circuit.NewWalkerState("test")
	artifact := &stubArtifact{typ: "test", confidence: 0.8, raw: map[string]any{}}

	if tr := edge.Evaluate(artifact, state); tr != nil {
		t.Error("edge fired without collector in state")
	}
}

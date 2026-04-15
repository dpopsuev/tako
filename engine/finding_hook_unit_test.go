package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestVetoHook_ImplementsHook(t *testing.T) {
	var _ Hook = &VetoHook{}
}

func TestHookingWalker_VetoIntercept(t *testing.T) {
	collector := &InMemoryFindingCollector{}
	_ = collector.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingError,
		NodeName: "nodeA",
		Domain:   "security",
	})

	vetoHook := NewVetoHook(collector)
	hooks := HookRegistry{}
	hooks.Register(vetoHook)

	inner := &stubWalker{
		state: circuit.NewWalkerState("test"),
	}

	hw := &hookingWalker{
		inner:     inner,
		nodeHooks: map[string][]string{"nodeA": {"finding-veto"}},
		hooks:     hooks,
	}

	vetoNode := &stubNode{
		name:     "nodeA",
		artifact: &stubArtifact{typ: "test", confidence: 0.85, raw: map[string]any{"result": "ok"}},
	}

	nc := circuit.NodeContext{WalkerState: circuit.NewWalkerState("test")}

	artifact, err := hw.Handle(context.Background(), vetoNode, nc)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if artifact.Confidence() != 0 {
		t.Errorf("Confidence() = %f, want 0 (veto should override)", artifact.Confidence())
	}
	if artifact.Type() != "test" {
		t.Errorf("Type() = %q, want %q (original type preserved)", artifact.Type(), "test")
	}
}

func TestHookingWalker_NoVeto_PassThrough(t *testing.T) {
	collector := &InMemoryFindingCollector{}
	_ = collector.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingWarning,
		NodeName: "nodeA",
	})

	vetoHook := NewVetoHook(collector)
	hooks := HookRegistry{}
	hooks.Register(vetoHook)

	inner := &stubWalker{
		state: circuit.NewWalkerState("test"),
	}

	hw := &hookingWalker{
		inner:     inner,
		nodeHooks: map[string][]string{"nodeA": {"finding-veto"}},
		hooks:     hooks,
	}

	vetoNode := &stubNode{
		name:     "nodeA",
		artifact: &stubArtifact{typ: "test", confidence: 0.85, raw: "data"},
	}

	nc := circuit.NodeContext{WalkerState: circuit.NewWalkerState("test")}

	artifact, err := hw.Handle(context.Background(), vetoNode, nc)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if artifact.Confidence() != 0.85 {
		t.Errorf("Confidence() = %f, want 0.85 (warning should not veto)", artifact.Confidence())
	}
}

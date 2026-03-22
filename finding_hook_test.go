package framework

import (
	"context"
	"testing"
)

func TestVetoHook_ImplementsHook(t *testing.T) {
	var _ Hook = &VetoHook{}
}

func TestHookingWalker_VetoIntercept(t *testing.T) {
	collector := &InMemoryFindingCollector{}
	_ = collector.Report(context.Background(), Finding{
		Severity: FindingError,
		NodeName: "nodeA",
		Domain:   "security",
	})

	vetoHook := NewVetoHook(collector)
	hooks := HookRegistry{}
	hooks.Register(vetoHook)

	inner := &mockWalker{
		artifact: &findingStubArtifact{typ: "test", confidence: 0.85, raw: map[string]any{"result": "ok"}},
	}

	hw := &hookingWalker{
		inner:     inner,
		nodeHooks: map[string][]string{"nodeA": {"finding-veto"}},
		hooks:     hooks,
	}

	node := &mockNode{name: "nodeA"}
	nc := NodeContext{WalkerState: NewWalkerState("test")}

	artifact, err := hw.Handle(context.Background(), node, nc)
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
	_ = collector.Report(context.Background(), Finding{
		Severity: FindingWarning,
		NodeName: "nodeA",
	})

	vetoHook := NewVetoHook(collector)
	hooks := HookRegistry{}
	hooks.Register(vetoHook)

	inner := &mockWalker{
		artifact: &findingStubArtifact{typ: "test", confidence: 0.85, raw: "data"},
	}

	hw := &hookingWalker{
		inner:     inner,
		nodeHooks: map[string][]string{"nodeA": {"finding-veto"}},
		hooks:     hooks,
	}

	node := &mockNode{name: "nodeA"}
	nc := NodeContext{WalkerState: NewWalkerState("test")}

	artifact, err := hw.Handle(context.Background(), node, nc)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if artifact.Confidence() != 0.85 {
		t.Errorf("Confidence() = %f, want 0.85 (warning should not veto)", artifact.Confidence())
	}
}

type mockWalker struct {
	artifact Artifact
	identity AgentIdentity
	state    *WalkerState
}

func (m *mockWalker) Identity() AgentIdentity                                       { return m.identity }
func (m *mockWalker) SetIdentity(id AgentIdentity)                                  { m.identity = id }
func (m *mockWalker) State() *WalkerState                                           { return m.state }
func (m *mockWalker) Handle(_ context.Context, _ Node, _ NodeContext) (Artifact, error) {
	return m.artifact, nil
}

type mockNode struct {
	name string
}

func (n *mockNode) Name() string              { return n.name }
func (n *mockNode) ElementAffinity() Element  { return "" }
func (n *mockNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return nil, nil
}

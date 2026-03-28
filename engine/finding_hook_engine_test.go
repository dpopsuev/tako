package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestVetoHook_FindingError_ReturnsVeto(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingError,
		NodeName: "login",
		Domain:   "security",
		Source:   "auditor",
		Message:  "credentials exposed",
	})

	hook := NewVetoHook(c)
	artifact := &stubArtifact{typ: "test", confidence: 0.9, raw: "data"}

	err := hook.Run(context.Background(), "login", artifact)
	if !errors.Is(err, circuit.ErrFindingVeto) {
		t.Errorf("Run() = %v, want ErrFindingVeto", err)
	}
}

func TestVetoHook_FindingWarning_NoVeto(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingWarning,
		NodeName: "login",
	})

	hook := NewVetoHook(c)
	artifact := &stubArtifact{typ: "test", confidence: 0.9, raw: "data"}

	err := hook.Run(context.Background(), "login", artifact)
	if err != nil {
		t.Errorf("Run() = %v, want nil (warning should not veto)", err)
	}
}

func TestVetoHook_DifferentNode_NoVeto(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingError,
		NodeName: "other-node",
	})

	hook := NewVetoHook(c)
	artifact := &stubArtifact{typ: "test", confidence: 0.9, raw: "data"}

	err := hook.Run(context.Background(), "login", artifact)
	if err != nil {
		t.Errorf("Run() = %v, want nil (error targets different node)", err)
	}
}

func TestVetoHook_NilArtifact_NoVeto(t *testing.T) {
	c := &InMemoryFindingCollector{}
	_ = c.Report(context.Background(), &circuit.Finding{
		Severity: circuit.FindingError,
		NodeName: "login",
	})

	hook := NewVetoHook(c)
	err := hook.Run(context.Background(), "login", nil)
	if err != nil {
		t.Errorf("Run() with nil artifact = %v, want nil", err)
	}
}

func TestVetoHook_Name(t *testing.T) {
	hook := NewVetoHook(&InMemoryFindingCollector{})
	if hook.Name() != "finding-veto" {
		t.Errorf("Name() = %q, want %q", hook.Name(), "finding-veto")
	}
}

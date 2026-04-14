package engine

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestCollectReferencedHooks_BeforeAndAfter(t *testing.T) {
	def := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{
			{Name: "a", Before: []string{"inject.data"}, After: []string{"store.result"}},
			{Name: "b", After: []string{"bridge.output"}},
		},
	}

	refs := collectReferencedHooks(def)
	if !refs["inject.data"] {
		t.Error("missing inject.data")
	}
	if !refs["store.result"] {
		t.Error("missing store.result")
	}
	if !refs["bridge.output"] {
		t.Error("missing bridge.output")
	}
	if len(refs) != 3 { //nolint:mnd // exactly 3 hooks declared
		t.Errorf("refs count = %d, want 3", len(refs))
	}
}

func TestCollectReferencedHooks_Empty(t *testing.T) {
	def := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{{Name: "a"}},
	}
	refs := collectReferencedHooks(def)
	if len(refs) != 0 {
		t.Errorf("refs count = %d, want 0", len(refs))
	}
}

func TestRegisteredHookNames_Sorted(t *testing.T) {
	reg := &GraphRegistries{
		Hooks: HookRegistry{
			"z.hook": NewHookFunc("z.hook", nil),
			"a.hook": NewHookFunc("a.hook", nil),
			"m.hook": NewHookFunc("m.hook", nil),
		},
	}

	names := registeredHookNames(reg)
	if len(names) != 3 { //nolint:mnd // 3 hooks
		t.Fatalf("names count = %d, want 3", len(names))
	}
	if names[0] != "a.hook" || names[1] != "m.hook" || names[2] != "z.hook" {
		t.Errorf("names not sorted: %v", names)
	}
}

func TestRegisteredHookNames_NilHooks(t *testing.T) {
	reg := &GraphRegistries{}
	names := registeredHookNames(reg)
	if names != nil {
		t.Errorf("expected nil for nil hooks, got %v", names)
	}
}

func TestRunBuildDiagnostics_NoPanic(t *testing.T) {
	// Given: a circuit with hooks that don't exist in registry
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "a", Before: []string{"missing.hook"}, After: []string{"also.missing"}},
		},
	}
	reg := &GraphRegistries{
		Hooks: HookRegistry{"registered.hook": NewHookFunc("registered.hook", nil)},
	}

	// When: run diagnostics (should not panic, only log warnings)
	runBuildDiagnostics(def, reg) // no assertion — just proves no panic
}

func TestRunBuildDiagnostics_EmptyRegistries(t *testing.T) {
	def := &circuit.CircuitDef{Circuit: "test"}
	reg := &GraphRegistries{}

	// Should not panic with empty registries
	runBuildDiagnostics(def, reg)
}

func TestDiagCircuitMediatorFallback_NoEndpoint(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Nodes:       []circuit.NodeDef{{Name: "sub", Instrument: "circuit", Action: "beta"}},
	}
	reg := &GraphRegistries{} // no mediator endpoint

	// Should be a no-op (no endpoint = no fallback possible)
	diagCircuitMediatorFallback(def, reg)
}

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// stubTransformerFor creates a named stub instrument for testing.
func stubTransformerFor(name string) Instrument {
	return InstrumentFunc(name, func(_ context.Context, _ *InstrumentContext) (any, error) {
		return "stub", nil
	})
}

// --- MergeComponents ---

func TestMergeComponents_SingleComponent(t *testing.T) {
	// Given: empty base + one component with a transformer
	base := &GraphRegistries{Instruments: InstrumentRegistry{}}
	comp := &Component{
		Namespace:   "alpha",
		Instruments: InstrumentRegistry{"llm": stubTransformerFor("llm")},
	}

	// When: merge
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}

	// Then: FQCN + short name registered
	if _, ok := merged.Instruments["alpha.llm"]; !ok {
		t.Error("missing FQCN alpha.llm")
	}
	if _, ok := merged.Instruments["llm"]; !ok {
		t.Error("missing short name llm")
	}
}

func TestMergeComponents_NamespaceCollision(t *testing.T) {
	// Given: base with existing FQCN + component with same FQCN
	base := &GraphRegistries{
		Instruments: InstrumentRegistry{"alpha.llm": stubTransformerFor("llm")},
	}
	comp := &Component{
		Namespace:   "alpha",
		Instruments: InstrumentRegistry{"llm": stubTransformerFor("llm2")},
	}

	// When: merge
	_, err := MergeComponents(base, comp)

	// Then: collision error
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !errors.Is(err, ErrInproc) {
		t.Errorf("want ErrInproc, got %v", err)
	}
}

func TestMergeComponents_ShortNamePreservesFirst(t *testing.T) {
	// Given: base with short name "llm" + component from different namespace also has "llm"
	existing := stubTransformerFor("existing")
	base := &GraphRegistries{
		Instruments: InstrumentRegistry{"llm": existing},
	}
	comp := &Component{
		Namespace:   "beta",
		Instruments: InstrumentRegistry{"llm": stubTransformerFor("new")},
	}

	// When: merge
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}

	// Then: short name "llm" still points to existing (first wins)
	if merged.Instruments["llm"] != existing {
		t.Error("short name should preserve first registration")
	}
	// But FQCN is registered
	if _, ok := merged.Instruments["beta.llm"]; !ok {
		t.Error("missing FQCN beta.llm")
	}
}

func TestMergeComponents_ExtractorCollision(t *testing.T) {
	base := &GraphRegistries{
		Extractors: ExtractorRegistry{"alpha.json": &JSONSchemaExtractor{}},
	}
	comp := &Component{
		Namespace:  "alpha",
		Extractors: ExtractorRegistry{"json": &JSONSchemaExtractor{}},
	}

	_, err := MergeComponents(base, comp)
	if err == nil {
		t.Fatal("expected extractor collision")
	}
	if !errors.Is(err, ErrExtractor) {
		t.Errorf("want ErrExtractor, got %v", err)
	}
}

func TestMergeComponents_HookCollision(t *testing.T) {
	base := &GraphRegistries{
		Hooks: HookRegistry{"alpha.store": NewHookFunc("store", nil)},
	}
	comp := &Component{
		Namespace: "alpha",
		Hooks:     HookRegistry{"store": NewHookFunc("store", nil)},
	}

	_, err := MergeComponents(base, comp)
	if err == nil {
		t.Fatal("expected hook collision")
	}
	if !errors.Is(err, ErrHook) {
		t.Errorf("want ErrHook, got %v", err)
	}
}

func TestMergeComponents_NilBase(t *testing.T) {
	// Given: base with nil registries
	base := &GraphRegistries{}
	comp := &Component{
		Namespace:   "alpha",
		Instruments: InstrumentRegistry{"llm": stubTransformerFor("llm")},
	}

	// When: merge (should initialize nil maps)
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}
	if _, ok := merged.Instruments["alpha.llm"]; !ok {
		t.Error("missing alpha.llm after nil base merge")
	}
}

func TestMergeComponents_PreservesBaseFields(t *testing.T) {
	// Given: base with Circuits and MediatorEndpoint
	base := &GraphRegistries{
		Circuits:         map[string]*circuit.CircuitDef{"test": {}},
		MediatorEndpoint: "http://localhost:9000",
	}
	comp := &Component{Namespace: "alpha"}

	// When: merge
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}

	// Then: non-handler fields preserved
	if len(merged.Circuits) != 1 {
		t.Error("Circuits not preserved")
	}
	if merged.MediatorEndpoint != "http://localhost:9000" {
		t.Error("MediatorEndpoint not preserved")
	}
}

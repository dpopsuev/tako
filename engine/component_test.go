package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// stubTransformerFor creates a named stub transformer for testing.
func stubTransformerFor(name string) Transformer {
	return TransformerFunc(name, func(_ context.Context, _ *TransformerContext) (any, error) {
		return "stub", nil
	})
}

// --- MergeComponents ---

func TestMergeComponents_SingleComponent(t *testing.T) {
	// Given: empty base + one component with a transformer
	base := &GraphRegistries{Transformers: TransformerRegistry{}}
	comp := &Component{
		Namespace:    "rca",
		Transformers: TransformerRegistry{"llm": stubTransformerFor("llm")},
	}

	// When: merge
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}

	// Then: FQCN + short name registered
	if _, ok := merged.Transformers["rca.llm"]; !ok {
		t.Error("missing FQCN rca.llm")
	}
	if _, ok := merged.Transformers["llm"]; !ok {
		t.Error("missing short name llm")
	}
}

func TestMergeComponents_NamespaceCollision(t *testing.T) {
	// Given: base with existing FQCN + component with same FQCN
	base := &GraphRegistries{
		Transformers: TransformerRegistry{"rca.llm": stubTransformerFor("llm")},
	}
	comp := &Component{
		Namespace:    "rca",
		Transformers: TransformerRegistry{"llm": stubTransformerFor("llm2")},
	}

	// When: merge
	_, err := MergeComponents(base, comp)

	// Then: collision error
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !errors.Is(err, ErrTransformer) {
		t.Errorf("want ErrTransformer, got %v", err)
	}
}

func TestMergeComponents_ShortNamePreservesFirst(t *testing.T) {
	// Given: base with short name "llm" + component from different namespace also has "llm"
	existing := stubTransformerFor("existing")
	base := &GraphRegistries{
		Transformers: TransformerRegistry{"llm": existing},
	}
	comp := &Component{
		Namespace:    "gnd",
		Transformers: TransformerRegistry{"llm": stubTransformerFor("new")},
	}

	// When: merge
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}

	// Then: short name "llm" still points to existing (first wins)
	if merged.Transformers["llm"] != existing {
		t.Error("short name should preserve first registration")
	}
	// But FQCN is registered
	if _, ok := merged.Transformers["gnd.llm"]; !ok {
		t.Error("missing FQCN gnd.llm")
	}
}

func TestMergeComponents_ExtractorCollision(t *testing.T) {
	base := &GraphRegistries{
		Extractors: ExtractorRegistry{"rca.json": &JSONSchemaExtractor{}},
	}
	comp := &Component{
		Namespace:  "rca",
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
		Hooks: HookRegistry{"rca.store": NewHookFunc("store", nil)},
	}
	comp := &Component{
		Namespace: "rca",
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
		Namespace:    "rca",
		Transformers: TransformerRegistry{"llm": stubTransformerFor("llm")},
	}

	// When: merge (should initialize nil maps)
	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatalf("MergeComponents: %v", err)
	}
	if _, ok := merged.Transformers["rca.llm"]; !ok {
		t.Error("missing rca.llm after nil base merge")
	}
}

func TestMergeComponents_PreservesBaseFields(t *testing.T) {
	// Given: base with Circuits and MediatorEndpoint
	base := &GraphRegistries{
		Circuits:         map[string]*circuit.CircuitDef{"test": {}},
		MediatorEndpoint: "http://localhost:9000",
	}
	comp := &Component{Namespace: "rca"}

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

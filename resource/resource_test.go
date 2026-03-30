package resource

import (
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// --- test helpers ---

type testResource struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func parseTestResource(data []byte) (*testResource, error) {
	return &testResource{Name: "test", Version: "1"}, nil
}

func validateTestResource(r *testResource) error {
	if r.Name == "" {
		return errors.New("name required")
	}
	return nil
}

func mergeTestResource(base, overlay *testResource) (*testResource, error) {
	result := *base
	if overlay.Name != "" {
		result.Name = overlay.Name
	}
	return &result, nil
}

func testYAML(kind string) []byte {
	return []byte("kind: " + kind + "\nmetadata:\n  name: test-resource\nversion: v1\n")
}

// --- KindHandler tests ---

func TestHandlerOf_Parse(t *testing.T) {
	h := NewHandler(circuit.KindScenario, parseTestResource, nil, nil)
	result, err := h.Parse([]byte("name: test"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tr, ok := result.(*testResource)
	if !ok {
		t.Fatal("expected *testResource")
	}
	if tr.Name != "test" {
		t.Errorf("Name = %q", tr.Name)
	}
}

func TestHandlerOf_Validate(t *testing.T) {
	h := NewHandler(circuit.KindScenario, parseTestResource, validateTestResource, nil)
	// Valid
	if err := h.Validate(&testResource{Name: "ok"}); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
	// Invalid
	if err := h.Validate(&testResource{Name: ""}); err == nil {
		t.Error("expected validation error")
	}
	// No validator
	h2 := NewHandler(circuit.KindScenario, parseTestResource, nil, nil)
	if err := h2.Validate(&testResource{}); err != nil {
		t.Errorf("nil validator should return nil, got %v", err)
	}
}

func TestHandlerOf_Merge(t *testing.T) {
	h := NewHandler(circuit.KindScenario, parseTestResource, nil, mergeTestResource)
	if !h.SupportsMerge() {
		t.Fatal("expected SupportsMerge=true")
	}

	result, err := h.Merge(&testResource{Name: "base"}, &testResource{Name: "overlay"})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	tr := result.(*testResource)
	if tr.Name != "overlay" {
		t.Errorf("Name = %q, want overlay", tr.Name)
	}
}

func TestHandlerOf_MergeNotSupported(t *testing.T) {
	h := NewHandler(circuit.KindScenario, parseTestResource, nil, nil)
	if h.SupportsMerge() {
		t.Fatal("expected SupportsMerge=false")
	}
	_, err := h.Merge(&testResource{}, &testResource{})
	if !errors.Is(err, ErrMergeNotSupported) {
		t.Errorf("expected ErrMergeNotSupported, got %v", err)
	}
}

func TestHandlerOf_TypeMismatch(t *testing.T) {
	h := NewHandler(circuit.KindScenario, parseTestResource, validateTestResource, mergeTestResource)
	if err := h.Validate("wrong type"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("Validate: expected ErrTypeMismatch, got %v", err)
	}
	_, err := h.Merge("wrong", &testResource{})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("Merge base: expected ErrTypeMismatch, got %v", err)
	}
	_, err = h.Merge(&testResource{}, "wrong")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("Merge overlay: expected ErrTypeMismatch, got %v", err)
	}
}

// --- KindRegistry tests ---

func TestKindRegistry_RegisterAndLookup(t *testing.T) {
	reg := NewKindRegistry()
	h := NewHandler(circuit.KindScenario, parseTestResource, nil, nil)
	reg.Register(h)

	if !reg.Has(circuit.KindScenario) {
		t.Error("expected Has=true")
	}
	if reg.Has(circuit.KindTuning) {
		t.Error("expected Has=false for unregistered")
	}
	if got := reg.Lookup(circuit.KindScenario); got == nil {
		t.Error("expected non-nil handler")
	}
	if got := reg.Lookup(circuit.KindTuning); got != nil {
		t.Error("expected nil for unregistered")
	}
}

func TestKindRegistry_Kinds(t *testing.T) {
	reg := NewKindRegistry()
	reg.Register(NewHandler(circuit.KindScorecard, parseTestResource, nil, nil))
	reg.Register(NewHandler(circuit.KindScenario, parseTestResource, nil, nil))

	kinds := reg.Kinds()
	if len(kinds) != 2 {
		t.Fatalf("Kinds = %d, want 2", len(kinds))
	}
	// Should be sorted
	if kinds[0] != circuit.KindScenario {
		t.Errorf("kinds[0] = %q, want scenario", kinds[0])
	}
	if kinds[1] != circuit.KindScorecard {
		t.Errorf("kinds[1] = %q, want scorecard", kinds[1])
	}
}

func TestKindRegistry_DuplicatePanics(t *testing.T) {
	reg := NewKindRegistry()
	h := NewHandler(circuit.KindScenario, parseTestResource, nil, nil)
	reg.Register(h)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	reg.Register(h)
}

// --- Load tests ---

func TestLoad_WithRegistry(t *testing.T) {
	reg := NewKindRegistry()
	reg.Register(NewHandler(circuit.KindScenario, parseTestResource, nil, nil))

	res, typed, err := Load(reg, testYAML("scenario"), "test.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Kind != circuit.KindScenario {
		t.Errorf("Kind = %q", res.Kind)
	}
	if res.Metadata.Name != "test-resource" {
		t.Errorf("Name = %q", res.Metadata.Name)
	}
	if res.Source != "test.yaml" {
		t.Errorf("Source = %q", res.Source)
	}
	if typed == nil {
		t.Error("expected non-nil typed object")
	}
}

func TestLoad_UnknownKind(t *testing.T) {
	reg := NewKindRegistry()
	_, _, err := Load(reg, testYAML("unknown-kind"), "test.yaml")
	if !errors.Is(err, ErrUnknownKind) {
		t.Errorf("expected ErrUnknownKind, got %v", err)
	}
}

func TestLoad_NoKind(t *testing.T) {
	reg := NewKindRegistry()
	_, _, err := Load(reg, []byte("name: test\n"), "test.yaml")
	if !errors.Is(err, ErrNoKindField) {
		t.Errorf("expected ErrNoKindField, got %v", err)
	}
}

// --- DefaultRegistry tests ---

func TestDefaultRegistry_AllKinds(t *testing.T) {
	reg := DefaultRegistry()
	expected := []circuit.Kind{
		// Typed handlers
		circuit.KindSchematic,
		circuit.KindStoreSchema,
		circuit.KindScorecard,
		circuit.KindReportTemplate,
		circuit.KindBoard,
		// Passthrough handlers
		circuit.KindComponent,
		circuit.KindScenario,
		circuit.KindSourcePack,
		circuit.KindVocabulary,
		circuit.KindHeuristicRules,
		circuit.KindTuning,
		circuit.KindArtifactSchema,
		circuit.KindDataset,
	}
	for _, k := range expected {
		if !reg.Has(k) {
			t.Errorf("DefaultRegistry missing kind %q", k)
		}
	}
	// Should have all 13 kinds (14 minus prompt which has no YAML yet)
	if got := len(reg.Kinds()); got != len(expected) {
		t.Errorf("Kinds() = %d, want %d", got, len(expected))
	}
}

func TestPassthroughHandler_Parse(t *testing.T) {
	h := NewPassthroughHandler(circuit.KindScenario)
	data := []byte("kind: scenario\nversion: v1\nmetadata:\n  name: ptp\ndescription: test scenario\n")
	result, err := h.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, ok := result.(*RawResource)
	if !ok {
		t.Fatal("expected *RawResource")
	}
	if raw.Kind != circuit.KindScenario {
		t.Errorf("Kind = %q", raw.Kind)
	}
	if raw.Name != "ptp" {
		t.Errorf("Name = %q", raw.Name)
	}
	if raw.Data["description"] != "test scenario" {
		t.Errorf("Data[description] = %v", raw.Data["description"])
	}
}

func TestPassthroughHandler_NoMerge(t *testing.T) {
	h := NewPassthroughHandler(circuit.KindTuning)
	if h.SupportsMerge() {
		t.Error("passthrough should not support merge")
	}
	_, err := h.Merge(nil, nil)
	if !errors.Is(err, ErrMergeNotSupported) {
		t.Errorf("expected ErrMergeNotSupported, got %v", err)
	}
}

func TestLoad_Passthrough(t *testing.T) {
	reg := DefaultRegistry()
	data := []byte("kind: scenario\nversion: v1\nmetadata:\n  name: my-scenario\ncases:\n  - id: C1\n")
	res, typed, err := Load(reg, data, "scenarios/test.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Kind != circuit.KindScenario {
		t.Errorf("Kind = %q", res.Kind)
	}
	if res.Metadata.Name != "my-scenario" {
		t.Errorf("Name = %q", res.Metadata.Name)
	}
	raw, ok := typed.(*RawResource)
	if !ok {
		t.Fatal("expected *RawResource for passthrough kind")
	}
	cases, ok := raw.Data["cases"]
	if !ok || cases == nil {
		t.Error("expected cases in raw data")
	}
}

// --- Diff tests ---

func TestDiff_DetectsChanges(t *testing.T) {
	a := &Resource{Raw: []byte("kind: test\nname: alpha\nvalue: 1\n")}
	b := &Resource{Raw: []byte("kind: test\nname: beta\nvalue: 1\n")}

	entries := Diff(a, b)
	found := false
	for _, e := range entries {
		if e.Path == "name" {
			found = true
			if e.A != "alpha" || e.B != "beta" {
				t.Errorf("name diff: %v → %v", e.A, e.B)
			}
		}
	}
	if !found {
		t.Error("expected diff on 'name' field")
	}
}

func TestDiff_Identical(t *testing.T) {
	a := &Resource{Raw: []byte("kind: test\nname: same\n")}
	b := &Resource{Raw: []byte("kind: test\nname: same\n")}

	entries := Diff(a, b)
	if len(entries) != 0 {
		t.Errorf("expected no diffs for identical resources, got %d", len(entries))
	}
}

// --- Resource.Summary tests ---

func TestResource_Summary(t *testing.T) {
	r := &Resource{
		Kind:     circuit.KindScorecard,
		Version:  "v2",
		Metadata: Metadata{Name: "rca"},
		Source:   "scorecards/rca.yaml",
	}
	got := r.Summary()
	if got != "scorecard/rca (v2) [scorecards/rca.yaml]" {
		t.Errorf("Summary = %q", got)
	}
}

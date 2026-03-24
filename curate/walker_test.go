package curate

import (
	"context"
	"testing"
)

// stubSource is a test EvidenceSource that returns fixed data for a known record ID.
type stubSource struct {
	recordID string
	data     []byte
	consumed bool
}

func (s *stubSource) Type() string          { return "stub" }
func (s *stubSource) CanHandle(ref string) bool {
	if s.consumed {
		return false
	}
	return ref == s.recordID
}
func (s *stubSource) Fetch(_ context.Context, _ string) (*RawEvidence, error) {
	s.consumed = true
	return &RawEvidence{
		SourceRef: s.recordID,
		MimeType:  "text/plain",
		Data:      s.data,
	}, nil
}

// stubExtractor is a test Extractor that returns fixed fields.
type stubExtractor struct {
	fields []Field
}

func (e *stubExtractor) Type() string { return "stub" }
func (e *stubExtractor) Extract(_ context.Context, _ *RawEvidence) ([]Field, error) {
	return e.fields, nil
}

func testSchema() Schema {
	nonEmpty := func(v any) bool {
		s, ok := v.(string)
		return ok && s != ""
	}
	return Schema{
		Name: "test-schema",
		Fields: []FieldSpec{
			{Name: "title", Requirement: Required, Validate: nonEmpty},
			{Name: "category", Requirement: Required, Validate: nonEmpty},
			{Name: "notes", Requirement: Optional},
		},
	}
}

func TestCurationWalker_FullCircuit(t *testing.T) {
	src := &stubSource{
		recordID: "R01",
		data:     []byte(`{"title": "test case", "category": "bug"}`),
	}
	ext := &stubExtractor{
		fields: []Field{
			{Name: "title", Value: "test case", Source: "stub"},
			{Name: "category", Value: "bug", Source: "stub"},
		},
	}

	walker := NewCurationWalker(CurationWalkerConfig{
		RecordID:   "R01",
		Schema:     testSchema(),
		Sources:    []EvidenceSource{src},
		Extractors: []Extractor{ext},
	})

	yamlData := loadTestYAML(t)
	g, err := BuildCurationGraph(yamlData)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	err = g.Walk(context.Background(), walker, "fetch")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if !walker.Promoted() {
		t.Error("record should be promoted after walking complete circuit")
	}

	rec := walker.Record()
	if rec.ID != "R01" {
		t.Errorf("ID = %q, want R01", rec.ID)
	}
	if !rec.Has("title") {
		t.Error("title field missing")
	}
	if !rec.Has("category") {
		t.Error("category field missing")
	}

	state := walker.State()
	if state.Status != "done" {
		t.Errorf("Status = %q, want done", state.Status)
	}

	expectedNodes := []string{"fetch", "extract", "validate", "enrich", "promote"}
	if len(state.History) != len(expectedNodes) {
		t.Fatalf("History len = %d, want %d", len(state.History), len(expectedNodes))
	}
	for i, want := range expectedNodes {
		if state.History[i].Node != want {
			t.Errorf("History[%d].Node = %q, want %q", i, state.History[i].Node, want)
		}
	}
}

func TestCurationWalker_IncompleteRecord(t *testing.T) {
	src := &stubSource{
		recordID: "R01",
		data:     []byte(`partial`),
	}
	ext := &stubExtractor{
		fields: []Field{
			{Name: "title", Value: "test case", Source: "stub"},
		},
	}

	walker := NewCurationWalker(CurationWalkerConfig{
		RecordID:   "R01",
		Schema:     testSchema(),
		Sources:    []EvidenceSource{src},
		Extractors: []Extractor{ext},
	})

	yamlData := loadTestYAML(t)
	g, err := BuildCurationGraph(yamlData)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	err = g.Walk(context.Background(), walker, "fetch")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if !walker.Promoted() {
		t.Error("incomplete record should still be promoted (circuit always completes)")
	}

	rec := walker.Record()
	if !rec.Has("title") {
		t.Error("title field should be present")
	}
	if rec.Has("category") {
		t.Error("category field should NOT be present (only title extracted)")
	}

	state := walker.State()
	if state.Status != "done" {
		t.Errorf("Status = %q, want done", state.Status)
	}
}

func TestCurationWalker_WithInitialRecord(t *testing.T) {
	initial := NewRecord("R01")
	initial.Set(Field{Name: "title", Value: "pre-existing", Source: "seed"})
	initial.Set(Field{Name: "category", Value: "known", Source: "seed"})

	src := &stubSource{recordID: "R01", data: []byte("x")}
	ext := &stubExtractor{fields: nil}

	walker := NewCurationWalker(CurationWalkerConfig{
		RecordID:      "R01",
		Schema:        testSchema(),
		Sources:       []EvidenceSource{src},
		Extractors:    []Extractor{ext},
		InitialRecord: &initial,
	})

	yamlData := loadTestYAML(t)
	g, err := BuildCurationGraph(yamlData)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	err = g.Walk(context.Background(), walker, "fetch")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if !walker.Promoted() {
		t.Error("record with all required fields should be promoted")
	}
	rec := walker.Record()
	f, _ := rec.Get("title")
	if f.Value != "pre-existing" {
		t.Errorf("title = %v, want pre-existing (should preserve initial)", f.Value)
	}
}

func TestCurationWalker_NoSources(t *testing.T) {
	walker := NewCurationWalker(CurationWalkerConfig{
		RecordID: "R01",
		Schema:   testSchema(),
	})

	yamlData := loadTestYAML(t)
	g, err := BuildCurationGraph(yamlData)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	err = g.Walk(context.Background(), walker, "fetch")
	if err != nil {
		t.Fatalf("Walk: %v (should handle empty sources gracefully)", err)
	}
}

func TestCurationWalker_InterfaceCompliance(t *testing.T) {
	walker := NewCurationWalker(CurationWalkerConfig{
		RecordID: "R01",
		Schema:   testSchema(),
	})

	id := walker.Identity()
	if id.PersonaName != "curator" {
		t.Errorf("PersonaName = %q, want curator", id.PersonaName)
	}

	state := walker.State()
	if state.ID != "R01" {
		t.Errorf("State.ID = %q, want R01", state.ID)
	}
	if state.Status != "running" {
		t.Errorf("State.Status = %q, want running", state.Status)
	}
}

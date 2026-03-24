package ingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Mock implementations ---

type mockSource struct {
	records []Record
	err     error
}

func (m *mockSource) Discover(_ context.Context, _ Config) ([]Record, error) {
	return m.records, m.err
}

type mockMatcher struct{}

func (m *mockMatcher) Match(records []Record) (matched, unmatched []Record) {
	for _, r := range records {
		if _, ok := r.Fields["pattern_id"]; ok {
			matched = append(matched, r)
		} else {
			unmatched = append(unmatched, r)
		}
	}
	return
}

type mockWriter struct {
	written []Candidate
}

func (m *mockWriter) Write(_ context.Context, candidates []Candidate) error {
	m.written = append(m.written, candidates...)
	return nil
}

// --- Pipeline tests ---

func TestRun_EmptySource(t *testing.T) {
	src := &mockSource{}
	summary, err := Run(context.Background(), Config{}, src, &mockMatcher{}, NewDedupIndex(), &mockWriter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Discovered != 0 {
		t.Errorf("Discovered = %d, want 0", summary.Discovered)
	}
}

func TestRun_FullPipeline(t *testing.T) {
	src := &mockSource{records: []Record{
		{ID: "1", DedupKey: "k1", Fields: map[string]any{"pattern_id": "P1"}},
		{ID: "2", DedupKey: "k2", Fields: map[string]any{"pattern_id": "P2"}},
		{ID: "3", DedupKey: "k3", Fields: map[string]any{"no_pattern": true}},
	}}

	dedup := NewDedupIndex()
	dedup.Add("k1")

	w := &mockWriter{}

	summary, err := Run(context.Background(), Config{}, src, &mockMatcher{}, dedup, w)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if summary.Discovered != 3 {
		t.Errorf("Discovered = %d, want 3", summary.Discovered)
	}
	if summary.Matched != 2 {
		t.Errorf("Matched = %d, want 2", summary.Matched)
	}
	if summary.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", summary.Duplicates)
	}
	if summary.Written != 1 {
		t.Errorf("Written = %d, want 1", summary.Written)
	}
	if len(w.written) != 1 {
		t.Fatalf("written = %d, want 1", len(w.written))
	}
	if w.written[0].PatternID != "P2" {
		t.Errorf("PatternID = %q, want P2", w.written[0].PatternID)
	}
	if w.written[0].Status != "candidate" {
		t.Errorf("Status = %q, want candidate", w.written[0].Status)
	}
}

func TestRun_AllDuplicates(t *testing.T) {
	src := &mockSource{records: []Record{
		{ID: "1", DedupKey: "k1", Fields: map[string]any{"pattern_id": "P1"}},
	}}
	dedup := NewDedupIndex()
	dedup.Add("k1")

	w := &mockWriter{}
	summary, err := Run(context.Background(), Config{}, src, &mockMatcher{}, dedup, w)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Written != 0 {
		t.Errorf("Written = %d, want 0", summary.Written)
	}
	if len(w.written) != 0 {
		t.Errorf("written = %d, want 0", len(w.written))
	}
}

func TestRun_NoMatches(t *testing.T) {
	src := &mockSource{records: []Record{
		{ID: "1", DedupKey: "k1", Fields: map[string]any{"no_pattern": true}},
	}}

	summary, err := Run(context.Background(), Config{}, src, &mockMatcher{}, NewDedupIndex(), &mockWriter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Matched != 0 {
		t.Errorf("Matched = %d, want 0", summary.Matched)
	}
	if summary.Written != 0 {
		t.Errorf("Written = %d, want 0", summary.Written)
	}
}

// --- DedupIndex tests ---

func TestDedupIndex_ContainsAdd(t *testing.T) {
	idx := NewDedupIndex()

	if idx.Contains("k1") {
		t.Error("empty index should not contain k1")
	}

	idx.Add("k1")
	if !idx.Contains("k1") {
		t.Error("expected k1 after Add")
	}
	if idx.Size() != 1 {
		t.Errorf("Size = %d, want 1", idx.Size())
	}
}

func TestDedupIndex_LoadFromDir(t *testing.T) {
	dir := t.TempDir()

	doc := map[string]any{"id": "C1", "dedup_key": "proj:100:1001"}
	data, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, "C1.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := LoadDedupIndex(dir)
	if err != nil {
		t.Fatalf("LoadDedupIndex: %v", err)
	}
	if !idx.Contains("proj:100:1001") {
		t.Error("expected key to be loaded")
	}
	if idx.Contains("proj:200:2001") {
		t.Error("unexpected key")
	}
}

func TestDedupIndex_LoadMissingDir(t *testing.T) {
	idx, err := LoadDedupIndex("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadDedupIndex: %v", err)
	}
	if idx.Size() != 0 {
		t.Errorf("Size = %d, want 0", idx.Size())
	}
}

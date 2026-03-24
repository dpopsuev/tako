package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type stubReader struct {
	ensureFn func(ctx context.Context, src Source) error
	searchFn func(ctx context.Context, src Source, query string, max int) ([]SearchResult, error)
	readFn   func(ctx context.Context, src Source, path string) ([]byte, error)
	listFn   func(ctx context.Context, src Source, root string, maxDepth int) ([]ContentEntry, error)
}

func (s *stubReader) Ensure(ctx context.Context, src Source) error {
	if s.ensureFn != nil {
		return s.ensureFn(ctx, src)
	}
	return nil
}
func (s *stubReader) Search(ctx context.Context, src Source, query string, max int) ([]SearchResult, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, src, query, max)
	}
	return nil, nil
}
func (s *stubReader) Read(ctx context.Context, src Source, path string) ([]byte, error) {
	if s.readFn != nil {
		return s.readFn(ctx, src, path)
	}
	return nil, nil
}
func (s *stubReader) List(ctx context.Context, src Source, root string, maxDepth int) ([]ContentEntry, error) {
	if s.listFn != nil {
		return s.listFn(ctx, src, root, maxDepth)
	}
	return nil, nil
}

func TestSourceReader_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	var _ SourceReader = (*stubReader)(nil)
}

func TestSourceReader_Ensure(t *testing.T) {
	t.Parallel()
	called := false
	r := &stubReader{ensureFn: func(_ context.Context, src Source) error {
		called = true
		if src.Name != "test" {
			return fmt.Errorf("unexpected source: %s", src.Name)
		}
		return nil
	}}
	err := r.Ensure(context.Background(), Source{Name: "test"})
	if err != nil {
		t.Fatalf("Ensure error: %v", err)
	}
	if !called {
		t.Error("Ensure was not called")
	}
}

func TestSourceReader_Search(t *testing.T) {
	t.Parallel()
	r := &stubReader{searchFn: func(_ context.Context, _ Source, query string, max int) ([]SearchResult, error) {
		return []SearchResult{
			{Source: "repo", Path: "main.go", Line: 42, Snippet: "func main()"},
		}, nil
	}}
	results, err := r.Search(context.Background(), Source{Name: "repo"}, "main", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].Line != 42 {
		t.Errorf("Line = %d, want 42", results[0].Line)
	}
}

func TestSearchResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	sr := SearchResult{Source: "repo", Path: "f.go", Line: 10, Snippet: "hello"}
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SearchResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != sr {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, sr)
	}
}

func TestContentEntry_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	ce := ContentEntry{Path: "/docs/readme.md", IsDir: false, Size: 1024}
	data, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ContentEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != ce {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, ce)
	}
}

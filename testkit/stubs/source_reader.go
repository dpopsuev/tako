package stubs

import (
	"context"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/toolkit"
)

// StubSourceReader implements toolkit.SourceReader with canned responses.
// Thread-safe, supports error injection and call tracking.
type StubSourceReader struct {
	mu          sync.Mutex
	readData    map[string][]byte           // keyed by source.Name + ":" + path
	searchData  map[string][]toolkit.SearchResult // keyed by source.Name + ":" + query
	listData    map[string][]toolkit.ContentEntry // keyed by source.Name + ":" + root
	ensuredSrcs []string
	err         error
	calls       []string
}

// NewStubSourceReader creates a source reader with canned read data.
// Keys in readData should be "sourceName:path" strings.
func NewStubSourceReader(readData map[string][]byte) *StubSourceReader {
	if readData == nil {
		readData = make(map[string][]byte)
	}
	return &StubSourceReader{
		readData:   readData,
		searchData: make(map[string][]toolkit.SearchResult),
		listData:   make(map[string][]toolkit.ContentEntry),
	}
}

// Ensure records that a source was ensured.
func (s *StubSourceReader) Ensure(_ context.Context, src *toolkit.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, "Ensure:"+src.Name)
	s.ensuredSrcs = append(s.ensuredSrcs, src.Name)

	if s.err != nil {
		return s.err
	}
	return nil
}

// Search returns canned search results for a source+query pair.
func (s *StubSourceReader) Search(_ context.Context, src *toolkit.Source, query string, _ int) ([]toolkit.SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := src.Name + ":" + query
	s.calls = append(s.calls, "Search:"+key)

	if s.err != nil {
		return nil, s.err
	}

	if results, ok := s.searchData[key]; ok {
		return results, nil
	}
	return nil, nil
}

// Read returns canned data for a source+path pair.
func (s *StubSourceReader) Read(_ context.Context, src *toolkit.Source, path string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := src.Name + ":" + path
	s.calls = append(s.calls, "Read:"+key)

	if s.err != nil {
		return nil, s.err
	}

	if data, ok := s.readData[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("stub: no data for %q", key)
}

// List returns canned content entries for a source+root pair.
func (s *StubSourceReader) List(_ context.Context, src *toolkit.Source, root string, _ int) ([]toolkit.ContentEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := src.Name + ":" + root
	s.calls = append(s.calls, "List:"+key)

	if s.err != nil {
		return nil, s.err
	}

	if entries, ok := s.listData[key]; ok {
		return entries, nil
	}
	return nil, nil
}

// SetError injects a global error for all methods.
func (s *StubSourceReader) SetError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

// WithSearchData sets canned search results for a source+query key.
func (s *StubSourceReader) WithSearchData(key string, results []toolkit.SearchResult) {
	s.mu.Lock()
	s.searchData[key] = results
	s.mu.Unlock()
}

// WithListData sets canned list entries for a source+root key.
func (s *StubSourceReader) WithListData(key string, entries []toolkit.ContentEntry) {
	s.mu.Lock()
	s.listData[key] = entries
	s.mu.Unlock()
}

// Calls returns a copy of all method call records.
func (s *StubSourceReader) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// EnsuredSources returns a copy of all source names passed to Ensure.
func (s *StubSourceReader) EnsuredSources() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.ensuredSrcs))
	copy(out, s.ensuredSrcs)
	return out
}

// Reset clears call tracking and injected errors.
func (s *StubSourceReader) Reset() {
	s.mu.Lock()
	s.calls = nil
	s.ensuredSrcs = nil
	s.err = nil
	s.mu.Unlock()
}

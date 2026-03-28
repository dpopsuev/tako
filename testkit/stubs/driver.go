package stubs

import (
	"context"
	"sync"

	"github.com/dpopsuev/origami/toolkit"
)

// StubDriver implements toolkit.Driver with canned responses.
// Thread-safe, supports error injection and call tracking.
// Composes StubSourceReader for read/search/list/ensure operations.
type StubDriver struct {
	mu     sync.Mutex
	kind   toolkit.SourceKind
	reader *StubSourceReader
	calls  []string
	err    error
}

// NewStubDriver creates a driver for the given source kind.
func NewStubDriver(kind toolkit.SourceKind) *StubDriver {
	return &StubDriver{
		kind:   kind,
		reader: NewStubSourceReader(nil),
	}
}

func (d *StubDriver) Handles() toolkit.SourceKind {
	return d.kind
}

func (d *StubDriver) Ensure(ctx context.Context, src *toolkit.Source) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "Ensure:"+src.Name)
	if d.err != nil {
		return d.err
	}
	return d.reader.Ensure(ctx, src)
}

func (d *StubDriver) Search(ctx context.Context, src *toolkit.Source, query string, maxResults int) ([]toolkit.SearchResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "Search:"+src.Name+":"+query)
	if d.err != nil {
		return nil, d.err
	}
	return d.reader.Search(ctx, src, query, maxResults)
}

func (d *StubDriver) Read(ctx context.Context, src *toolkit.Source, path string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "Read:"+src.Name+":"+path)
	if d.err != nil {
		return nil, d.err
	}
	return d.reader.Read(ctx, src, path)
}

func (d *StubDriver) List(ctx context.Context, src *toolkit.Source, root string, maxDepth int) ([]toolkit.ContentEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "List:"+src.Name+":"+root)
	if d.err != nil {
		return nil, d.err
	}
	return d.reader.List(ctx, src, root, maxDepth)
}

// SetError injects an error returned by all subsequent operations.
func (d *StubDriver) SetError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.err = err
}

// Calls returns a copy of the call log.
func (d *StubDriver) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.calls))
	copy(out, d.calls)
	return out
}

// Reset clears call tracking and errors.
func (d *StubDriver) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = nil
	d.err = nil
	d.reader.Reset()
}

// WithReadData sets canned read data. Key format: "sourceName:path".
func (d *StubDriver) WithReadData(key string, data []byte) {
	d.reader.mu.Lock()
	defer d.reader.mu.Unlock()
	d.reader.readData[key] = data
}

// WithSearchData sets canned search results. Key format: "sourceName:query".
func (d *StubDriver) WithSearchData(key string, results []toolkit.SearchResult) {
	d.reader.WithSearchData(key, results)
}

// WithListData sets canned list entries. Key format: "sourceName:root".
func (d *StubDriver) WithListData(key string, entries []toolkit.ContentEntry) {
	d.reader.WithListData(key, entries)
}

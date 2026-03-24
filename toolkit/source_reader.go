package toolkit

import "context"

// SourceReader is the unified source access interface. Consuming schematics
// depend on this interface without knowing whether it is backed by an
// in-process router, an MCP subprocess, or a container.
type SourceReader interface {
	Ensure(ctx context.Context, src Source) error
	Search(ctx context.Context, src Source, query string, maxResults int) ([]SearchResult, error)
	Read(ctx context.Context, src Source, path string) ([]byte, error)
	List(ctx context.Context, src Source, root string, maxDepth int) ([]ContentEntry, error)
}

// SearchResult represents a single search hit from a data source.
type SearchResult struct {
	Source  string `json:"source"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet"`
}

// ContentEntry represents a file or document in a source listing.
type ContentEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

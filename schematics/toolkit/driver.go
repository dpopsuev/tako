package toolkit

import "context"

// Driver implements GND access for a specific SourceKind.
// Drivers are registered with the access router.
type Driver interface {
	Handles() SourceKind
	Ensure(ctx context.Context, src Source) error
	Search(ctx context.Context, src Source, query string, maxResults int) ([]SearchResult, error)
	Read(ctx context.Context, src Source, path string) ([]byte, error)
	List(ctx context.Context, src Source, root string, maxDepth int) ([]ContentEntry, error)
}

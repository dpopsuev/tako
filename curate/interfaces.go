package curate

import "context"

// RawEvidence is unprocessed data fetched from a source.
type RawEvidence struct {
	SourceRef string `json:"source_ref"`
	MimeType  string `json:"mime_type,omitempty"`
	Data      []byte `json:"data"`
}

// EvidenceSource fetches raw evidence from an external system (API, file, URL).
// Implementations are domain-specific (e.g. JiraSource, GitHubPRSource).
type EvidenceSource interface {
	Type() string
	CanHandle(ref string) bool
	Fetch(ctx context.Context, ref string) (*RawEvidence, error)
}

// Extractor parses raw evidence into structured fields.
// Implementations may be rule-based or AI-driven.
type Extractor interface {
	Type() string
	Extract(ctx context.Context, raw *RawEvidence) ([]Field, error)
}

// Store persists and retrieves Datasets.
// The canonical implementation is a JSON file store.
type Store interface {
	List(ctx context.Context) ([]string, error)
	Load(ctx context.Context, name string) (*Dataset, error)
	Save(ctx context.Context, d *Dataset) error
}

package docs

import "errors"

var (
	// ErrDocsEnsure is returned for: docs ensure
	ErrDocsEnsure = errors.New("docs ensure")

	// ErrHTTP is returned for: HTTP
	ErrHTTP = errors.New("HTTP")
)

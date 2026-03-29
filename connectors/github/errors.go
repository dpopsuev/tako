package github

import "errors"

var (
	// ErrCannotParseGitURI is returned for: cannot parse git URI
	ErrCannotParseGitURI = errors.New("cannot parse git URI")

	// ErrPathTraversal is returned for: path traversal
	ErrPathTraversal = errors.New("path traversal")
)

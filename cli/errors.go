package cli

import "errors"

var (
	// ErrLintErrors is returned when lint finds errors.
	ErrLintErrors = errors.New("lint errors found")
)

package domainserve

import "errors"

var (
	// ErrUnknownFile is returned for: unknown file
	ErrUnknownFile = errors.New("unknown file")

	// ErrUnknownSection is returned for: unknown section
	ErrUnknownSection = errors.New("unknown section")

	// ErrUnknownKey is returned for: unknown key
	ErrUnknownKey = errors.New("unknown key")
)

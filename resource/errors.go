package resource

import "errors"

// Sentinel errors for resource operations.
var (
	// ErrUnknownKind is returned when a kind has no registered handler.
	ErrUnknownKind = errors.New("unknown resource kind")

	// ErrMergeNotSupported is returned when a kind does not support overlay merging.
	ErrMergeNotSupported = errors.New("merge not supported for this kind")

	// ErrTypeMismatch is returned when a parsed value doesn't match the handler's expected type.
	ErrTypeMismatch = errors.New("resource type mismatch")

	// ErrNoKindField is returned when YAML data has no kind: field.
	ErrNoKindField = errors.New("YAML data has no kind field")

	// ErrKindNameCollision is returned when a custom kind name collides with a built-in kind.
	ErrKindNameCollision = errors.New("kind name collision")
)

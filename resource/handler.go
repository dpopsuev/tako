package resource

import (
	"github.com/dpopsuev/origami/circuit"
)

// KindHandler defines the operations available for a registered kind.
// Each handler is registered in a KindRegistry and dispatched by kind value.
type KindHandler interface {
	// Kind returns the kind constant this handler serves.
	Kind() circuit.Kind

	// Parse deserializes raw YAML bytes into the kind's typed Go struct.
	Parse(data []byte) (any, error)

	// Validate checks a parsed resource for structural/semantic correctness.
	// Returns nil if valid or if no validator is registered.
	Validate(parsed any) error

	// Merge applies an overlay onto a base resource.
	// Returns ErrMergeNotSupported if the kind does not support overlays.
	Merge(base, overlay any) (any, error)

	// SupportsMerge returns true if this kind supports overlay merging.
	SupportsMerge() bool
}

// HandlerOf is a generic typed adapter that wraps concrete parse/validate/merge
// functions into a KindHandler. Handler authors provide typed functions;
// the adapter handles the any↔T conversion.
type HandlerOf[T any] struct {
	kind     circuit.Kind
	parse    func([]byte) (*T, error)
	validate func(*T) error
	merge    func(base, overlay *T) (*T, error)
}

// NewHandler creates a typed KindHandler for the given kind.
// Pass nil for validate or merge if the kind doesn't support them.
func NewHandler[T any](
	kind circuit.Kind,
	parse func([]byte) (*T, error),
	validate func(*T) error,
	merge func(base, overlay *T) (*T, error),
) *HandlerOf[T] {
	return &HandlerOf[T]{
		kind:     kind,
		parse:    parse,
		validate: validate,
		merge:    merge,
	}
}

func (h *HandlerOf[T]) Kind() circuit.Kind { return h.kind }

func (h *HandlerOf[T]) Parse(data []byte) (any, error) {
	return h.parse(data)
}

func (h *HandlerOf[T]) Validate(parsed any) error {
	if h.validate == nil {
		return nil
	}
	typed, ok := parsed.(*T)
	if !ok {
		return ErrTypeMismatch
	}
	return h.validate(typed)
}

func (h *HandlerOf[T]) Merge(base, overlay any) (any, error) {
	if h.merge == nil {
		return nil, ErrMergeNotSupported
	}
	typedBase, ok := base.(*T)
	if !ok {
		return nil, ErrTypeMismatch
	}
	typedOverlay, ok := overlay.(*T)
	if !ok {
		return nil, ErrTypeMismatch
	}
	return h.merge(typedBase, typedOverlay)
}

func (h *HandlerOf[T]) SupportsMerge() bool { return h.merge != nil }

package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

// NewJSONExtractor parses JSON bytes into a typed Go struct.
// Generic function — cannot be aliased via var, so forwarded explicitly.
func NewJSONExtractor[T any](name string) Extractor {
	return engine.NewJSONExtractor[T](name)
}

var (
	NewCodeBlockExtractor = engine.NewCodeBlockExtractor
	NewLineSplitExtractor = engine.NewLineSplitExtractor
)

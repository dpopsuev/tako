//nolint:dupl // registry pattern intentionally repeated per type
package handler

import (
	"context"
	"fmt"
	"strings"
)

// Extractor pulls structured data from raw output.
type Extractor interface {
	Name() string
	Extract(ctx context.Context, input any) (any, error)
}

// ExtractorRegistry maps extractor names to implementations.
type ExtractorRegistry map[string]Extractor

// Get returns the extractor registered under name.
func (r ExtractorRegistry) Get(name string) (Extractor, error) {
	if r == nil {
		return nil, ErrExtractorRegistryIsNil
	}
	if e, ok := r[name]; ok {
		return e, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, e := range r {
			if strings.HasSuffix(k, suffix) {
				return e, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %q not registered", ErrExtractor, name)
}

// Register adds an extractor. Panics on duplicate.
func (r ExtractorRegistry) Register(ext Extractor) {
	if _, exists := r[ext.Name()]; exists {
		panic(fmt.Sprintf("duplicate extractor registration: %q", ext.Name()))
	}
	r[ext.Name()] = ext
}

//nolint:dupl // registry pattern intentionally repeated per type
package handler

import (
	"context"
	"fmt"
	"strings"
)

// Renderer formats artifacts for display.
type Renderer interface {
	Name() string
	Render(ctx context.Context, data any) (string, error)
}

// RendererRegistry maps renderer names to implementations.
type RendererRegistry map[string]Renderer

// Get returns the renderer registered under name.
func (r RendererRegistry) Get(name string) (Renderer, error) {
	if r == nil {
		return nil, ErrRendererRegistryIsNil
	}
	if rnd, ok := r[name]; ok {
		return rnd, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, rnd := range r {
			if strings.HasSuffix(k, suffix) {
				return rnd, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %q not registered", ErrRenderer, name)
}

// Register adds a renderer. Panics on duplicate.
func (r RendererRegistry) Register(rnd Renderer) {
	if _, exists := r[rnd.Name()]; exists {
		panic(fmt.Sprintf("duplicate renderer registration: %q", rnd.Name()))
	}
	r[rnd.Name()] = rnd
}

package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/circuit"
)

// Hook runs side effects before/after nodes.
type Hook interface {
	Name() string
	Run(ctx context.Context, nodeName string, artifact circuit.Artifact) error
}

// HookRegistry maps hook names to implementations.
type HookRegistry map[string]Hook

// Get returns the hook registered under name.
func (r HookRegistry) Get(name string) (Hook, error) {
	if r == nil {
		return nil, ErrHookRegistryIsNil
	}
	if h, ok := r[name]; ok {
		return h, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, h := range r {
			if strings.HasSuffix(k, suffix) {
				return h, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %q not registered", ErrHook, name)
}

// Register adds a hook. Panics on duplicate.
func (r HookRegistry) Register(h Hook) {
	if _, exists := r[h.Name()]; exists {
		panic(fmt.Sprintf("duplicate hook registration: %q", h.Name()))
	}
	r[h.Name()] = h
}

// HookFunc is a convenience adapter that turns a plain function into a Hook.
type HookFunc struct {
	HookName string
	Fn       func(ctx context.Context, nodeName string, artifact circuit.Artifact) error
}

// NewHookFunc creates a HookFunc.
func NewHookFunc(name string, fn func(ctx context.Context, nodeName string, artifact circuit.Artifact) error) *HookFunc {
	return &HookFunc{HookName: name, Fn: fn}
}

func (h *HookFunc) Name() string { return h.HookName }
func (h *HookFunc) Run(ctx context.Context, nodeName string, artifact circuit.Artifact) error {
	return h.Fn(ctx, nodeName, artifact)
}

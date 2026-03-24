package toolkit

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// NewContextInjector creates a before-hook that extracts the walker context
// and calls fn with it. This eliminates the repeated boilerplate of
// WalkerStateFromContext + nil check in every inject-style hook.
//
// The hook is a no-op if the walker state is not available in context.
func NewContextInjector(name string, fn func(walkerCtx map[string]any)) engine.Hook {
	return engine.NewHookFunc(name, func(ctx context.Context, _ string, _ circuit.Artifact) error {
		ws := engine.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		fn(ws.Context)
		return nil
	})
}

// NewContextInjectorErr is like NewContextInjector but the injector function
// can return an error to abort the walk.
func NewContextInjectorErr(name string, fn func(ctx context.Context, walkerCtx map[string]any) error) engine.Hook {
	return engine.NewHookFunc(name, func(ctx context.Context, _ string, _ circuit.Artifact) error {
		ws := engine.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		return fn(ctx, ws.Context)
	})
}

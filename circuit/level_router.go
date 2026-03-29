package circuit

import (
	"context"
	"log/slog"
)

// LevelRouter is an slog.Handler that applies per-component minimum log levels.
// Components not in the map use the default level.
type LevelRouter struct {
	levels       map[string]slog.Level
	defaultLevel slog.Level
	inner        slog.Handler
	component    string // cached from WithAttrs
}

// NewLevelRouter creates a handler that filters log records by component.
// The levels map keys should be LogComponent* constants.
func NewLevelRouter(levels map[string]slog.Level, defaultLevel slog.Level, inner slog.Handler) *LevelRouter {
	return &LevelRouter{
		levels:       levels,
		defaultLevel: defaultLevel,
		inner:        inner,
	}
}

// Enabled returns true broadly; actual filtering happens in Handle
// after the component attribute is inspected.
func (r *LevelRouter) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle inspects the component attribute and drops records below
// the component's minimum level.
//
//nolint:gocritic // slog.Handler interface requires value receiver for Record
func (r *LevelRouter) Handle(ctx context.Context, rec slog.Record) error {
	comp := r.component
	if comp == "" {
		rec.Attrs(func(a slog.Attr) bool {
			if a.Key == LogKeyComponent {
				comp = a.Value.String()
				return false
			}
			return true
		})
	}

	minLevel := r.defaultLevel
	if comp != "" {
		if lvl, ok := r.levels[comp]; ok {
			minLevel = lvl
		}
	}

	if rec.Level < minLevel {
		return nil
	}

	return r.inner.Handle(ctx, rec)
}

// WithAttrs caches the component attribute if present, then delegates.
func (r *LevelRouter) WithAttrs(attrs []slog.Attr) slog.Handler {
	comp := r.component
	for _, a := range attrs {
		if a.Key == LogKeyComponent {
			comp = a.Value.String()
			break
		}
	}
	return &LevelRouter{
		levels:       r.levels,
		defaultLevel: r.defaultLevel,
		inner:        r.inner.WithAttrs(attrs),
		component:    comp,
	}
}

// WithGroup delegates to the inner handler.
func (r *LevelRouter) WithGroup(name string) slog.Handler {
	return &LevelRouter{
		levels:       r.levels,
		defaultLevel: r.defaultLevel,
		inner:        r.inner.WithGroup(name),
		component:    r.component,
	}
}

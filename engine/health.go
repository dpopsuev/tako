package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/origami/circuit"
)

// ErrComponentUnhealthy is returned when a component's health check fails.
var ErrComponentUnhealthy = fmt.Errorf("component unhealthy")

// HealthChecker is optionally implemented by components that can verify
// their runtime prerequisites (network reachability, credentials, etc.).
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// CheckComponentHealth runs health checks on all components that implement
// HealthChecker. Returns the first failure wrapped with ErrComponentUnhealthy.
// Components without a Health field are silently skipped.
func CheckComponentHealth(ctx context.Context, components []*Component) error {
	for _, comp := range components {
		if comp.Health == nil {
			slog.DebugContext(ctx, circuit.LogComponentHealthSkipped,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentRegistry),
				slog.Any(circuit.LogKeyName, comp.Name),
			)
			continue
		}

		slog.DebugContext(ctx, circuit.LogComponentHealthCheck,
			slog.Any(circuit.LogKeyComponent, circuit.LogComponentRegistry),
			slog.Any(circuit.LogKeyName, comp.Name),
		)

		if err := comp.Health.HealthCheck(ctx); err != nil {
			slog.WarnContext(ctx, circuit.LogComponentUnhealthy,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentRegistry),
				slog.Any(circuit.LogKeyName, comp.Name),
				slog.Any(circuit.LogKeyError, err),
			)
			return fmt.Errorf("%w: %s: %w", ErrComponentUnhealthy, comp.Name, err)
		}

		slog.DebugContext(ctx, circuit.LogComponentHealthy,
			slog.Any(circuit.LogKeyComponent, circuit.LogComponentRegistry),
			slog.Any(circuit.LogKeyName, comp.Name),
		)
	}

	slog.InfoContext(ctx, circuit.LogAllComponentsHealthy,
		slog.Any(circuit.LogKeyComponent, circuit.LogComponentRegistry),
		slog.Any(circuit.LogKeyCount, len(components)),
	)
	return nil
}

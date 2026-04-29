package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/broker"
	"github.com/dpopsuev/tangle/signal"
)

const hookName = "tako-observability"

// observabilityHook implements broker.SpawnHook and broker.PerformHook.
// Emits lifecycle signals on the circuit's signal.Bus so consumers get
// agent observability for free.
type observabilityHook struct {
	bus signal.Bus
	log *slog.Logger
}

// newObservabilityHook creates a hook that emits agent lifecycle signals.
func newObservabilityHook(bus signal.Bus) *observabilityHook {
	return &observabilityHook{
		bus: bus,
		log: slog.Default(),
	}
}

func (h *observabilityHook) Name() string { return hookName }

const (
	logKeyRole     = "role"
	logKeyModel    = "model"
	logKeyProvider = "provider"
)

// PreSpawn logs the spawn attempt.
func (h *observabilityHook) PreSpawn(ctx context.Context, config troupe.AgentConfig) error {
	h.log.InfoContext(ctx, "agent spawn requested",
		slog.String(logKeyRole, config.Role),
		slog.String(logKeyModel, config.Model),
		slog.String(logKeyProvider, config.Provider))
	return nil
}

// PostSpawn emits a signal on spawn success or failure.
func (h *observabilityHook) PostSpawn(_ context.Context, config troupe.AgentConfig, _ troupe.Agent, err error) {
	if err != nil {
		h.emit(signal.EventWorkerError, map[string]string{
			"role":  config.Role,
			"error": err.Error(),
			"phase": "spawn",
		})
		return
	}
	h.emit(signal.EventWorkerStarted, map[string]string{
		"role":     config.Role,
		"model":    config.Model,
		"provider": config.Provider,
	})
}

// PrePerform is a no-op — we don't block on perform.
func (h *observabilityHook) PrePerform(_ context.Context, _ string) error {
	return nil
}

// PostPerform emits a signal with duration and response size.
func (h *observabilityHook) PostPerform(_ context.Context, _, response string, err error) {
	meta := map[string]string{
		"response_len": fmt.Sprintf("%d", len(response)),
	}
	if err != nil {
		meta["error"] = err.Error()
		h.emit(signal.EventWorkerError, meta)
		return
	}
	h.emit(signal.EventWorkerDone, meta)
}

func (h *observabilityHook) emit(event string, meta map[string]string) {
	if h.bus == nil {
		return
	}
	meta["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	h.bus.Emit(&signal.Signal{
		Event: event,
		Agent: signal.AgentWorker,
		Meta:  meta,
	})
}

// Compile-time interface checks.
var (
	_ broker.SpawnHook   = (*observabilityHook)(nil)
	_ broker.PerformHook = (*observabilityHook)(nil)
)

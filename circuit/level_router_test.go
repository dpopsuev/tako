package circuit_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestLevelRouter_PerComponentFiltering(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	router := circuit.NewLevelRouter(map[string]slog.Level{
		circuit.LogComponentMuxDispatch:    slog.LevelDebug,
		circuit.LogComponentCircuitSession: slog.LevelWarn,
	}, slog.LevelInfo, inner)

	logger := slog.New(router)

	// mux_dispatch at Debug — should pass (Debug >= Debug)
	logger.DebugContext(context.Background(), "debug msg",
		slog.String(circuit.LogKeyComponent, circuit.LogComponentMuxDispatch),
	)
	if buf.Len() == 0 {
		t.Error("mux_dispatch Debug should pass")
	}

	// circuit_session at Info — should be dropped (Info < Warn)
	buf.Reset()
	logger.InfoContext(context.Background(), "info msg",
		slog.String(circuit.LogKeyComponent, circuit.LogComponentCircuitSession),
	)
	if buf.Len() != 0 {
		t.Error("circuit_session Info should be dropped")
	}

	// circuit_session at Warn — should pass (Warn >= Warn)
	buf.Reset()
	logger.WarnContext(context.Background(), "warn msg",
		slog.String(circuit.LogKeyComponent, circuit.LogComponentCircuitSession),
	)
	if buf.Len() == 0 {
		t.Error("circuit_session Warn should pass")
	}
}

func TestLevelRouter_DefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	router := circuit.NewLevelRouter(map[string]slog.Level{}, slog.LevelWarn, inner)
	logger := slog.New(router)

	// Unknown component at Info — should be dropped (Info < Warn default)
	logger.InfoContext(context.Background(), "info msg",
		slog.String(circuit.LogKeyComponent, "unknown"),
	)
	if buf.Len() != 0 {
		t.Error("unknown component Info should be dropped with Warn default")
	}

	// Unknown component at Warn — should pass
	buf.Reset()
	logger.WarnContext(context.Background(), "warn msg",
		slog.String(circuit.LogKeyComponent, "unknown"),
	)
	if buf.Len() == 0 {
		t.Error("unknown component Warn should pass with Warn default")
	}
}

func TestLevelRouter_NoComponent(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	router := circuit.NewLevelRouter(map[string]slog.Level{}, slog.LevelInfo, inner)
	logger := slog.New(router)

	// No component attr — uses default
	logger.DebugContext(context.Background(), "debug msg")
	if buf.Len() != 0 {
		t.Error("no component Debug should be dropped with Info default")
	}

	buf.Reset()
	logger.InfoContext(context.Background(), "info msg")
	if buf.Len() == 0 {
		t.Error("no component Info should pass with Info default")
	}
}

func TestLevelRouter_WithAttrsCaching(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	router := circuit.NewLevelRouter(map[string]slog.Level{
		circuit.LogComponentMuxDispatch: slog.LevelDebug,
	}, slog.LevelWarn, inner)

	// Create a sub-logger with component pre-set via WithAttrs
	dispatchLogger := slog.New(router.WithAttrs([]slog.Attr{
		slog.String(circuit.LogKeyComponent, circuit.LogComponentMuxDispatch),
	}))

	// Debug should pass — component cached from WithAttrs
	dispatchLogger.DebugContext(context.Background(), "debug msg")
	if buf.Len() == 0 {
		t.Error("cached mux_dispatch Debug should pass")
	}
}

func TestLevelRouter_ComposesWithSafeHandler(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	safe := circuit.NewSafeHandler(jsonHandler)
	router := circuit.NewLevelRouter(map[string]slog.Level{
		circuit.LogComponentMuxDispatch: slog.LevelDebug,
	}, slog.LevelInfo, safe)

	logger := slog.New(router)

	// Debug with sensitive key — should pass AND be redacted
	logger.DebugContext(context.Background(), "test",
		slog.String(circuit.LogKeyComponent, circuit.LogComponentMuxDispatch),
		slog.String("token", "secret-value"),
	)
	if buf.Len() == 0 {
		t.Fatal("mux_dispatch Debug should pass")
	}
	m := parseLogLine(t, &buf)
	if !strings.Contains(m["token"].(string), "REDACTED") {
		t.Errorf("token should be redacted: %v", m["token"])
	}
}

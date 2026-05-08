package assemble

import (
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type SlogListener struct{}

var _ cerebrum.ContextListener = SlogListener{}

func (SlogListener) OnContext(phase string, turn int, distance float64) {
	slog.Info("turn",
		slog.Int("turn", turn),
		slog.String("phase", phase),
		slog.Float64("distance", distance))
}

func (SlogListener) OnToolCall(name string, _ []byte) {
	slog.Info("tool.call", slog.String("name", name))
}

func (SlogListener) OnToolResult(name string, _ []byte, elapsed time.Duration) {
	slog.Info("tool.result", slog.String("name", name), slog.Duration("elapsed", elapsed))
}

func (SlogListener) OnSealed(id string, distance float64, turns int, _ string) {
	slog.Info("sealed",
		slog.String("molecule", id),
		slog.Float64("distance", distance),
		slog.Int("turns", turns))
}

func (SlogListener) OnError(turn int, err error) {
	slog.Warn("error", slog.Int("turn", turn), slog.Any("error", err))
}

func (SlogListener) OnToken(_ string) {}

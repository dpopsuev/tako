package engine

// Category: Execution — observer implementations.

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dpopsuev/origami/circuit"
)

// logObserver writes walk events as structured slog lines.
type logObserver struct {
	Logger *slog.Logger
}

// NewLogObserver creates a WalkObserver that logs events using the given logger.
// If logger is nil, slog.Default() is used.
func NewLogObserver(logger *slog.Logger) circuit.WalkObserver {
	return &logObserver{Logger: logger}
}

func (o *logObserver) OnEvent(e *circuit.WalkEvent) {
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}

	attrs := []slog.Attr{
		slog.String(circuit.LogKeyEvent, string(e.Type)),
	}
	if e.Node != "" {
		attrs = append(attrs, slog.String(circuit.LogKeyNode, e.Node))
	}
	if e.Walker != "" {
		attrs = append(attrs, slog.String(circuit.LogKeyWalker, e.Walker))
	}
	if e.Edge != "" {
		attrs = append(attrs, slog.String(circuit.LogKeyEdge, e.Edge))
	}
	if e.Elapsed > 0 {
		attrs = append(attrs, slog.Duration(circuit.LogKeyElapsedDur, e.Elapsed))
	}
	if e.Error != nil {
		attrs = append(attrs, slog.String(circuit.LogKeyError, e.Error.Error()))
	}
	if e.Metadata != nil {
		for k, v := range e.Metadata {
			attrs = append(attrs, slog.Group(circuit.LogKeyMeta, slog.Any(k, v)))
		}
	}

	if e.Error != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "walk", attrs...)
	} else {
		logger.LogAttrs(context.Background(), slog.LevelInfo, "walk", attrs...)
	}
}

// TraceCollector accumulates walk events in memory for post-walk analysis.
// Safe for concurrent use.
type TraceCollector struct {
	mu     sync.Mutex
	events []circuit.WalkEvent
}

func (t *TraceCollector) OnEvent(e *circuit.WalkEvent) {
	t.mu.Lock()
	t.events = append(t.events, *e)
	t.mu.Unlock()
}

// Events returns a copy of all collected events.
func (t *TraceCollector) Events() []circuit.WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]circuit.WalkEvent, len(t.events))
	copy(out, t.events)
	return out
}

// Reset clears collected events.
func (t *TraceCollector) Reset() {
	t.mu.Lock()
	t.events = nil
	t.mu.Unlock()
}

// EventsOfType returns only events matching the given type.
func (t *TraceCollector) EventsOfType(typ circuit.WalkEventType) []circuit.WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []circuit.WalkEvent
	for _, e := range t.events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

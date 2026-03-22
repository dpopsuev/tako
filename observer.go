package framework

// Category: Processing & Support — aliases to core/ package.
// Implementations (logObserver, NewLogObserver, TraceCollector) stay here.

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dpopsuev/origami/core"
)

type WalkEventType = core.WalkEventType

const (
	EventNodeEnter        = core.EventNodeEnter
	EventNodeExit         = core.EventNodeExit
	EventEdgeEvaluate     = core.EventEdgeEvaluate
	EventTransition       = core.EventTransition
	EventWalkerSwitch     = core.EventWalkerSwitch
	EventFanOutStart      = core.EventFanOutStart
	EventFanOutEnd        = core.EventFanOutEnd
	EventWalkComplete     = core.EventWalkComplete
	EventWalkError        = core.EventWalkError
	EventWalkInterrupted  = core.EventWalkInterrupted
	EventWalkResumed      = core.EventWalkResumed
	EventCheckpointSaved  = core.EventCheckpointSaved
	EventProviderFallback = core.EventProviderFallback
	EventCircuitOpen      = core.EventCircuitOpen
	EventCircuitClose     = core.EventCircuitClose
	EventRateLimit        = core.EventRateLimit
	EventThermalWarning   = core.EventThermalWarning
	EventDelegateStart    = core.EventDelegateStart
	EventDelegateEnd      = core.EventDelegateEnd
)

type WalkEvent = core.WalkEvent
type WalkObserver = core.WalkObserver
type WalkObserverFunc = core.WalkObserverFunc
type MultiObserver = core.MultiObserver

// emitEvent is a helper to safely emit an event to a possibly-nil observer.
// Duplicated from core/ because it is unexported and used by root-package code.
func emitEvent(obs WalkObserver, e WalkEvent) {
	if obs != nil {
		obs.OnEvent(e)
	}
}

// logObserver writes walk events as structured slog lines.
type logObserver struct {
	Logger *slog.Logger
}

// NewLogObserver creates a WalkObserver that logs events using the given logger.
// If logger is nil, slog.Default() is used.
func NewLogObserver(logger *slog.Logger) WalkObserver {
	return &logObserver{Logger: logger}
}

func (o *logObserver) OnEvent(e WalkEvent) {
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}

	attrs := []slog.Attr{
		slog.String("event", string(e.Type)),
	}
	if e.Node != "" {
		attrs = append(attrs, slog.String("node", e.Node))
	}
	if e.Walker != "" {
		attrs = append(attrs, slog.String("walker", e.Walker))
	}
	if e.Edge != "" {
		attrs = append(attrs, slog.String("edge", e.Edge))
	}
	if e.Elapsed > 0 {
		attrs = append(attrs, slog.Duration("elapsed", e.Elapsed))
	}
	if e.Error != nil {
		attrs = append(attrs, slog.String("error", e.Error.Error()))
	}
	if e.Metadata != nil {
		for k, v := range e.Metadata {
			attrs = append(attrs, slog.Any("meta."+k, v))
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
	events []WalkEvent
}

func (t *TraceCollector) OnEvent(e WalkEvent) {
	t.mu.Lock()
	t.events = append(t.events, e)
	t.mu.Unlock()
}

// Events returns a copy of all collected events.
func (t *TraceCollector) Events() []WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]WalkEvent, len(t.events))
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
func (t *TraceCollector) EventsOfType(typ WalkEventType) []WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []WalkEvent
	for _, e := range t.events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

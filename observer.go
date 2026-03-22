package framework

// Category: Processing & Support — aliases to core/ and engine/ packages.

import (
	"log/slog"

	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/engine"
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

// TraceCollector accumulates walk events in memory for post-walk analysis.
type TraceCollector = engine.TraceCollector

// NewLogObserver creates a WalkObserver that logs events using the given logger.
func NewLogObserver(logger *slog.Logger) WalkObserver { return engine.NewLogObserver(logger) }

// emitEvent is a helper to safely emit an event to a possibly-nil observer.
// Duplicated from core/ because it is unexported and used by root-package code.
func emitEvent(obs WalkObserver, e WalkEvent) {
	if obs != nil {
		obs.OnEvent(e)
	}
}

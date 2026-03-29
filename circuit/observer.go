package circuit

// Category: Processing & Support

import "time"

// WalkEventType classifies walk events for filtering and routing.
type WalkEventType string

const (
	EventNodeEnter        WalkEventType = "node_enter"
	EventNodeExit         WalkEventType = "node_exit"
	EventEdgeEvaluate     WalkEventType = "edge_evaluate"
	EventTransition       WalkEventType = "transition"
	EventWalkerSwitch     WalkEventType = "walker_switch"
	EventFanOutStart      WalkEventType = "fan_out_start"
	EventFanOutEnd        WalkEventType = "fan_out_end"
	EventWalkComplete     WalkEventType = "walk_complete"
	EventWalkError        WalkEventType = "walk_error"
	EventWalkInterrupted  WalkEventType = "walk_interrupted"
	EventWalkResumed      WalkEventType = "walk_resumed"
	EventCheckpointSaved  WalkEventType = "checkpoint_saved"
	EventProviderFallback WalkEventType = "provider_fallback"
	EventCircuitOpen      WalkEventType = "circuit_open"
	EventCircuitClose     WalkEventType = "circuit_close"
	EventRateLimit        WalkEventType = "rate_limit"
	EventThermalWarning   WalkEventType = "thermal_warning"
	EventDelegateStart    WalkEventType = "delegate_start"
	EventDelegateEnd      WalkEventType = "delegate_end"
)

// WalkEvent is a single observation from a graph walk. The Metadata map
// is the forward-compatible extension point — new fields go there
// without breaking the struct.
type WalkEvent struct {
	Type     WalkEventType
	Node     string
	Walker   string
	Edge     string
	Artifact Artifact
	Elapsed  time.Duration
	Error    error
	Metadata map[string]any
}

// WalkObserver receives events during a graph walk. Single-method
// design (like http.Handler) so adding new event types never breaks
// existing observers.
type WalkObserver interface {
	OnEvent(*WalkEvent)
}

// WalkObserverFunc adapts a plain function to the WalkObserver interface.
type WalkObserverFunc func(*WalkEvent)

func (f WalkObserverFunc) OnEvent(e *WalkEvent) { f(e) }

// MultiObserver fans out events to multiple observers.
type MultiObserver []WalkObserver

func (m MultiObserver) OnEvent(e *WalkEvent) {
	for _, obs := range m {
		obs.OnEvent(e)
	}
}

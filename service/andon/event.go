package andon

import "time"

type EventType string

const (
	NodeEnter    EventType = "node_enter"
	NodeExit     EventType = "node_exit"
	EdgeEvaluate EventType = "edge_evaluate"
	Transition   EventType = "transition"
	AgentSwitch  EventType = "agent_switch"
	FanOutStart  EventType = "fan_out_start"
	FanOutEnd    EventType = "fan_out_end"
	WalkComplete EventType = "walk_complete"
	WalkError    EventType = "walk_error"
	Interrupted  EventType = "interrupted"
	Resumed      EventType = "resumed"
	Checkpoint   EventType = "checkpoint"
	Fallback     EventType = "fallback"
	BreakerOpen  EventType = "breaker_open"
	BreakerClose EventType = "breaker_close"
	RateLimit    EventType = "rate_limit"
	Thermal      EventType = "thermal_warning"
	DelegateStart EventType = "delegate_start"
	DelegateEnd  EventType = "delegate_end"
)

type Event struct {
	Type     EventType
	Node     string
	Agent    string
	Edge     string
	Elapsed  time.Duration
	Error    error
	Metadata map[string]any
}

type Observer interface {
	OnEvent(*Event)
}

type ObserverFunc func(*Event)

func (f ObserverFunc) OnEvent(e *Event) { f(e) }

type MultiObserver []Observer

func (m MultiObserver) OnEvent(e *Event) {
	for _, obs := range m {
		obs.OnEvent(e)
	}
}

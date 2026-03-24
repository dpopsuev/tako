package kami

import (
	"time"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

// SessionObserver wires MCP session lifecycle events to a Kami server's
// EventBridge and CircuitStore. It satisfies the mcpconfig.SessionObserver
// interface without mcpconfig importing kami or view.
type SessionObserver struct {
	server *Server
	store  *view.CircuitStore
	bridge *EventBridge
}

// NewSessionObserver creates a SessionObserver backed by the given Kami server.
func NewSessionObserver(srv *Server) *SessionObserver {
	return &SessionObserver{server: srv}
}

// OnSessionCreate builds a fresh CircuitStore and EventBridge for the
// new session, replacing any previous ones.
func (o *SessionObserver) OnSessionCreate(def *circuit.CircuitDef, bus signal.Bus) {
	if o.bridge != nil {
		o.bridge.Close()
	}
	st := view.NewCircuitStore(def)
	br := NewEventBridge(bus)
	br.StartPolling(100 * time.Millisecond)
	o.server.SetStore(st)
	o.store = st
	o.bridge = br
}

// OnStepDispatched emits a node-enter event for the given case/step.
func (o *SessionObserver) OnStepDispatched(caseID, step string) {
	if o.store != nil {
		o.store.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeEnter,
			Node:   step,
			Walker: caseID,
		})
	}
}

// OnStepCompleted emits a node-exit event for the given case/step.
func (o *SessionObserver) OnStepCompleted(caseID, step string, _ int64) {
	if o.store != nil {
		o.store.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeExit,
			Node:   step,
			Walker: caseID,
		})
	}
}

// OnCircuitDone emits a walk-complete event.
func (o *SessionObserver) OnCircuitDone() {
	if o.store != nil {
		o.store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventWalkComplete,
		})
	}
}

// OnSessionEnd emits a walk-complete event when the session is torn down.
func (o *SessionObserver) OnSessionEnd() {
	if o.store != nil {
		o.store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventWalkComplete,
		})
	}
}

// Close releases the bridge and store resources.
func (o *SessionObserver) Close() {
	if o.bridge != nil {
		o.bridge.Close()
		o.bridge = nil
	}
	if o.store != nil {
		o.store.Close()
		o.store = nil
	}
}

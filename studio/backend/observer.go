// Package backend provides the Studio server: a WalkObserver that records
// circuit events and serves them via REST API + SSE for the Visual Editor.
package backend

import (
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// StudioEvent is a serializable circuit event stored in the event store.
type StudioEvent struct {
	ID        int            `json:"id"`
	RunID     string         `json:"run_id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"ts"`
	Node      string         `json:"node,omitempty"`
	Edge      string         `json:"edge,omitempty"`
	Walker    string         `json:"walker,omitempty"`
	ElapsedMs int64          `json:"elapsed_ms,omitempty"`
	Error     string         `json:"error,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// RunInfo tracks metadata about a circuit run.
type RunInfo struct {
	ID        string    `json:"id"`
	Circuit  string    `json:"circuit"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Status    string    `json:"status"` // "running", "completed", "error"
	NodeCount int       `json:"node_count"`
	EdgeCount int       `json:"edge_count"`
}

// StudioObserver implements circuit.WalkObserver and records events
// for the Visual Editor's REST API.
type StudioObserver struct {
	store   *EventStore
	runID   string
	circuit string
}

// NewStudioObserver creates an observer that records to the given store.
func NewStudioObserver(store *EventStore, runID, circuit string) *StudioObserver {
	return &StudioObserver{
		store:    store,
		runID:    runID,
		circuit: circuit,
	}
}

// OnEvent implements circuit.WalkObserver.
func (o *StudioObserver) OnEvent(we circuit.WalkEvent) {
	evt := StudioEvent{
		RunID:     o.runID,
		Type:      string(we.Type),
		Timestamp: time.Now().UTC(),
		Node:      we.Node,
		Edge:      we.Edge,
		Walker:    we.Walker,
		Metadata:  we.Metadata,
	}
	if we.Elapsed > 0 {
		evt.ElapsedMs = we.Elapsed.Milliseconds()
	}
	if we.Error != nil {
		evt.Error = we.Error.Error()
	}

	o.store.Append(evt)
}

// RunID returns the observer's run ID.
func (o *StudioObserver) RunID() string { return o.runID }

// Circuit returns the observer's circuit name.
func (o *StudioObserver) Circuit() string { return o.circuit }

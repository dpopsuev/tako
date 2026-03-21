package framework

// Category: Processing & Support

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// TraceLevel classifies trace events by verbosity.
type TraceLevel string

const (
	LevelInfo  TraceLevel = "info"  // server-level: session, step dispatch/submit
	LevelDebug TraceLevel = "debug" // walker-level: node enter/exit, edge eval, hooks
	LevelTrace TraceLevel = "trace" // transformer-level: prompt/artifact detail
)

// TraceEvent is a single entry in the execution trace JSONL stream.
// Both SignalBus (info) and WalkObserver (debug/trace) events are
// unified into this type.
type TraceEvent struct {
	Timestamp   string         `json:"ts"`
	Level       TraceLevel     `json:"level"`
	Event       string         `json:"event"`
	Node        string         `json:"node,omitempty"`
	Walker      string         `json:"walker,omitempty"`
	Edge        string         `json:"edge,omitempty"`
	CaseID      string         `json:"case_id,omitempty"`
	Step        string         `json:"step,omitempty"`
	Agent       string         `json:"agent,omitempty"`
	ElapsedMs   int64          `json:"elapsed_ms,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"meta,omitempty"`
	ArtifactRef string         `json:"artifact_ref,omitempty"`
}

// TraceRecorder writes a unified JSONL execution trace. It implements
// WalkObserver (for debug/trace events from the walker) and provides
// HandleSignal for info events from the SignalBus.
//
// Usage:
//
//	recorder, _ := NewTraceRecorder("runs/s-123/trace.jsonl")
//	defer recorder.Close()
//	bus.SetOnEmit(func(s dispatch.Signal) {
//	    recorder.HandleSignal(s.Timestamp, s.Event, s.Agent, s.CaseID, s.Step, s.Meta)
//	})
//	batchWalkConfig.Observer = recorder
type TraceRecorder struct {
	mu         sync.Mutex
	w          *bufio.Writer
	file       *os.File
	eventCount int
}

// NewTraceRecorder creates a recorder that writes JSONL to path.
// The parent directory must exist.
func NewTraceRecorder(path string) (*TraceRecorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &TraceRecorder{
		w:    bufio.NewWriterSize(f, 8192),
		file: f,
	}, nil
}

// OnEvent implements WalkObserver. Maps WalkEvents to TraceEvents at
// debug level (or trace for artifact details on node exit).
//
// Case ID is derived from e.Walker (which BatchWalk sets to the case ID
// via NewProcessWalker(bc.ID)), not from mutable recorder state. This
// makes OnEvent safe for concurrent walkers sharing one recorder.
func (r *TraceRecorder) OnEvent(e WalkEvent) {
	te := TraceEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     LevelDebug,
		Event:     string(e.Type),
		Node:      e.Node,
		Walker:    e.Walker,
		Edge:      e.Edge,
		CaseID:    e.Walker, // Walker name IS the case ID in BatchWalk
	}

	if e.Elapsed > 0 {
		te.ElapsedMs = e.Elapsed.Milliseconds()
	}
	if e.Error != nil {
		te.Error = e.Error.Error()
	}
	if e.Metadata != nil {
		te.Metadata = e.Metadata
	}

	r.write(te)

	// Emit a separate trace-level event for artifact details on node exit.
	// The node_exit event itself stays at debug so debug queries see the
	// complete node lifecycle (enter + exit with elapsed time).
	if e.Type == EventNodeExit && e.Artifact != nil {
		artEvent := TraceEvent{
			Timestamp: te.Timestamp,
			Level:     LevelTrace,
			Event:     "artifact_detail",
			Node:      e.Node,
			Walker:    e.Walker,
			CaseID:    e.Walker,
			ElapsedMs: te.ElapsedMs,
		}
		r.write(artEvent)
	}
}

// HandleSignal maps a SignalBus signal to a TraceEvent at info level.
// Accepts individual fields to avoid an import cycle with dispatch/.
// Wire with: bus.SetOnEmit(func(s Signal) { recorder.HandleSignal(s.Timestamp, s.Event, ...) })
func (r *TraceRecorder) HandleSignal(ts, event, agent, caseID, step string, meta map[string]string) {
	te := TraceEvent{
		Timestamp: ts,
		Level:     LevelInfo,
		Event:     event,
		CaseID:    caseID,
		Step:      step,
		Agent:     agent,
	}
	if len(meta) > 0 {
		te.Metadata = make(map[string]any, len(meta))
		for k, v := range meta {
			te.Metadata[k] = v
		}
	}
	r.write(te)
}

// Close flushes the buffer and closes the file.
func (r *TraceRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.w != nil {
		r.w.Flush()
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// EventCount returns the number of trace events written so far.
func (r *TraceRecorder) EventCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eventCount
}

func (r *TraceRecorder) write(te TraceEvent) {
	data, err := json.Marshal(te)
	if err != nil {
		return
	}
	data = append(data, '\n')
	r.mu.Lock()
	r.w.Write(data)
	r.eventCount++
	r.mu.Unlock()
}

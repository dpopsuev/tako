package observe

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/dpopsuev/tako/service/andon"
)

type TraceLevel string

const (
	LevelInfo  TraceLevel = "info"
	LevelDebug TraceLevel = "debug"
	LevelTrace TraceLevel = "trace"
)

type TraceEvent struct {
	Timestamp   string         `json:"ts"`
	Level       TraceLevel     `json:"level"`
	Event       string         `json:"event"`
	Node        string         `json:"node,omitempty"`
	Agent       string         `json:"agent,omitempty"`
	Edge        string         `json:"edge,omitempty"`
	ElapsedMs   int64          `json:"elapsed_ms,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"meta,omitempty"`
}

type TraceRecorder struct {
	mu         sync.Mutex
	w          *bufio.Writer
	file       *os.File
	eventCount int
}

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

func (r *TraceRecorder) OnEvent(e *andon.Event) {
	te := TraceEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     LevelDebug,
		Event:     string(e.Type),
		Node:      e.Node,
		Agent:     e.Agent,
		Edge:      e.Edge,
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
	r.write(&te)
}

func (r *TraceRecorder) HandleSignal(ts, event, agent, step string, meta map[string]string) {
	te := TraceEvent{
		Timestamp: ts,
		Level:     LevelInfo,
		Event:     event,
		Agent:     agent,
	}
	if len(meta) > 0 {
		te.Metadata = make(map[string]any, len(meta))
		for k, v := range meta {
			te.Metadata[k] = v
		}
	}
	r.write(&te)
}

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

func (r *TraceRecorder) EventCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eventCount
}

func (r *TraceRecorder) write(te *TraceEvent) {
	data, err := json.Marshal(te)
	if err != nil {
		return
	}
	data = append(data, '\n')
	r.mu.Lock()
	_, _ = r.w.Write(data)
	r.eventCount++
	r.mu.Unlock()
}

// LoadTrace reads a JSONL trace file and returns all events.
func LoadTrace(path string) ([]TraceEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open trace: %w", err)
	}
	defer f.Close()

	var events []TraceEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	for scanner.Scan() {
		var te TraceEvent
		if err := json.Unmarshal(scanner.Bytes(), &te); err != nil {
			continue
		}
		events = append(events, te)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan trace: %w", err)
	}
	return events, nil
}

type TraceSummary struct {
	TotalEvents int            `json:"total_events"`
	Nodes       []NodeSummary  `json:"nodes"`
	Errors      []TraceEvent   `json:"errors,omitempty"`
	Duration    time.Duration  `json:"duration"`
	EventCounts map[string]int `json:"event_counts"`
}

type NodeSummary struct {
	Name      string `json:"name"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Entered   bool   `json:"entered"`
	Exited    bool   `json:"exited"`
}

func Summarize(events []TraceEvent) TraceSummary {
	s := TraceSummary{
		TotalEvents: len(events),
		EventCounts: make(map[string]int),
	}
	nodeMap := make(map[string]*NodeSummary)

	for i := range events {
		e := &events[i]
		s.EventCounts[e.Event]++
		if e.Error != "" {
			s.Errors = append(s.Errors, *e)
		}
		switch e.Event {
		case string(andon.NodeEnter):
			if _, ok := nodeMap[e.Node]; !ok {
				nodeMap[e.Node] = &NodeSummary{Name: e.Node}
			}
			nodeMap[e.Node].Entered = true
		case string(andon.NodeExit):
			if ns, ok := nodeMap[e.Node]; ok {
				ns.Exited = true
				ns.ElapsedMs = e.ElapsedMs
			}
		}
	}

	s.Nodes = make([]NodeSummary, 0, len(nodeMap))
	for _, ns := range nodeMap {
		s.Nodes = append(s.Nodes, *ns)
	}
	sort.Slice(s.Nodes, func(i, j int) bool {
		return s.Nodes[i].Name < s.Nodes[j].Name
	})

	if len(events) >= 2 {
		first, _ := time.Parse(time.RFC3339Nano, events[0].Timestamp)
		last, _ := time.Parse(time.RFC3339Nano, events[len(events)-1].Timestamp)
		s.Duration = last.Sub(first)
	}
	return s
}

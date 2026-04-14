package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

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
			continue // skip malformed lines
		}
		events = append(events, te)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan trace: %w", err)
	}
	return events, nil
}

// TraceSummary provides aggregate statistics from a trace.
type TraceSummary struct {
	TotalEvents int            `json:"total_events"`
	Nodes       []NodeSummary  `json:"nodes"`
	Errors      []TraceEvent   `json:"errors,omitempty"`
	Duration    time.Duration  `json:"duration"`
	EventCounts map[string]int `json:"event_counts"`
}

// NodeSummary captures per-node timing from a trace.
type NodeSummary struct {
	Name      string `json:"name"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Entered   bool   `json:"entered"`
	Exited    bool   `json:"exited"`
}

// Summarize produces aggregate statistics from trace events.
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
		case "node_enter":
			if _, ok := nodeMap[e.Node]; !ok {
				nodeMap[e.Node] = &NodeSummary{Name: e.Node}
			}
			nodeMap[e.Node].Entered = true
		case "node_exit":
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

	// Duration from first to last event.
	if len(events) >= 2 {
		first, _ := time.Parse(time.RFC3339Nano, events[0].Timestamp)
		last, _ := time.Parse(time.RFC3339Nano, events[len(events)-1].Timestamp)
		s.Duration = last.Sub(first)
	}

	return s
}

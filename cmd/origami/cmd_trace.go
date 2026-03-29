package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

const labelUnknown = "unknown"

func traceCmd(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	stateDir := fs.String("state-dir", "", "state directory (default: .origami/state or $ORIGAMI_STATE_DIR)")
	runID := fs.String("run", "", "run ID (default: most recent)")
	v := fs.Bool("v", false, "include debug events")
	vv := fs.Bool("vv", false, "include debug and trace events")
	caseFilter := fs.String("case", "", "filter by CaseID")
	nodeFilter := fs.String("node", "", "filter by Node")
	errorsOnly := fs.Bool("errors", false, "show only events with non-empty Error")
	format := fs.String("format", "text", "output format: text, json")
	follow := fs.Bool("follow", false, "annotate delegation events and inline child traces when available")
	if err := fs.Parse(args); err != nil {
		return err
	}

	sd := *stateDir
	if sd == "" {
		sd = os.Getenv("ORIGAMI_STATE_DIR")
	}
	if sd == "" {
		sd = defaultStateDir
	}

	tracePath, err := resolveTracePath(sd, *runID)
	if err != nil {
		return err
	}

	events, err := readTraceEvents(tracePath)
	if err != nil {
		return err
	}

	// Determine max level to show.
	maxLevel := engine.LevelInfo
	if *v {
		maxLevel = engine.LevelDebug
	}
	if *vv {
		maxLevel = engine.LevelTrace
	}

	filtered := filterEvents(events, maxLevel, *caseFilter, *nodeFilter, *errorsOnly)

	if *follow {
		filtered = annotateDelegations(filtered, sd)
	}

	switch *format {
	case formatJSON:
		return renderTraceJSON(w, filtered)
	case formatText:
		return renderTraceText(w, filtered)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownFormat, *format)
	}
}

func resolveTracePath(stateDir, runID string) (string, error) {
	return resolveRunFile(stateDir, runID, "trace.jsonl")
}

func readTraceEvents(path string) ([]engine.TraceEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}
	defer f.Close()

	var events []engine.TraceEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev engine.TraceEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read trace file: %w", err)
	}
	return events, nil
}

func traceLevelRank(l engine.TraceLevel) int {
	switch l {
	case engine.LevelInfo:
		return 0
	case engine.LevelDebug:
		return 1
	case engine.LevelTrace:
		return 2
	default:
		return 3
	}
}

func filterEvents(events []engine.TraceEvent, maxLevel engine.TraceLevel, caseID, node string, errorsOnly bool) []engine.TraceEvent {
	maxRank := traceLevelRank(maxLevel)
	out := make([]engine.TraceEvent, 0, len(events))
	for i := range events {
		ev := &events[i]
		if traceLevelRank(ev.Level) > maxRank {
			continue
		}
		if caseID != "" && ev.CaseID != caseID {
			continue
		}
		if node != "" && ev.Node != node {
			continue
		}
		if errorsOnly && ev.Error == "" {
			continue
		}
		out = append(out, events[i])
	}
	return out
}

func renderTraceJSON(w io.Writer, events []engine.TraceEvent) error {
	enc := json.NewEncoder(w)
	for i := range events {
		if err := enc.Encode(events[i]); err != nil {
			return err
		}
	}
	return nil
}

func renderTraceText(w io.Writer, events []engine.TraceEvent) error {
	for i := range events {
		ev := &events[i]
		ts := formatTimestamp(ev.Timestamp)
		level := strings.ToUpper(string(ev.Level))

		var parts []string
		parts = append(parts, fmt.Sprintf("%-8s %-5s %-24s", ts, level, ev.Event))

		if ev.CaseID != "" {
			parts = append(parts, ev.CaseID)
		}
		if ev.Node != "" {
			parts = append(parts, ev.Node)
		}

		line := strings.Join(parts, " ")

		// Append key=value pairs for extra fields.
		var extras []string
		if ev.Walker != "" {
			extras = append(extras, fmt.Sprintf("walker=%s", ev.Walker))
		}
		if ev.Edge != "" {
			extras = append(extras, fmt.Sprintf("edge=%s", ev.Edge))
		}
		if ev.Step != "" {
			extras = append(extras, fmt.Sprintf("step=%s", ev.Step))
		}
		if ev.Agent != "" {
			extras = append(extras, fmt.Sprintf("agent=%s", ev.Agent))
		}
		if ev.ElapsedMs > 0 {
			extras = append(extras, fmt.Sprintf("elapsed=%dms", ev.ElapsedMs))
		}
		if ev.Error != "" {
			extras = append(extras, fmt.Sprintf("error=%q", ev.Error))
		}
		if ev.ArtifactRef != "" {
			extras = append(extras, fmt.Sprintf("artifact=%s", ev.ArtifactRef))
		}
		for k, v := range ev.Metadata {
			extras = append(extras, fmt.Sprintf("%s=%v", k, v))
		}

		if len(extras) > 0 {
			line += "   " + strings.Join(extras, " ")
		}

		fmt.Fprintln(w, line)
	}
	return nil
}

// formatTimestamp extracts the time portion (HH:MM:SS) from an RFC3339 timestamp.
func formatTimestamp(ts string) string {
	// Try to find a "T" separator and extract HH:MM:SS.
	if idx := strings.IndexByte(ts, 'T'); idx >= 0 {
		rest := ts[idx+1:]
		// Take up to 8 characters for HH:MM:SS.
		if len(rest) >= 8 {
			return rest[:8]
		}
		return rest
	}
	// Fallback: return as-is truncated.
	if len(ts) > 8 {
		return ts[:8]
	}
	return ts
}

// annotateDelegations annotates delegate_start/delegate_end events with
// [DELEGATION: <circuit_type>] markers and inlines child trace events when
// a matching child run is found in the same StateDir.
//
// For cross-service traces (mediator routing to remote backends), use the MCP
// get_trace tool with follow_delegations=true, which fetches child traces
// from remote backends via the mediator. The CLI only resolves local children.
func annotateDelegations(events []engine.TraceEvent, stateDir string) []engine.TraceEvent {
	// Build a trace_id → run directory index for child trace lookup.
	childByTraceID := indexChildRuns(stateDir)

	var out []engine.TraceEvent
	for i := range events {
		ct, _ := events[i].Metadata[circuit.ExtraKeyCircuitType].(string)
		switch events[i].Event {
		case "delegate_start":
			label := ct
			if label == "" {
				label = labelUnknown
			}
			ev := annotateEvent(&events[i], fmt.Sprintf("[DELEGATION START: %s]", label))
			out = append(out, ev)

			// Try to inline child trace events.
			traceID, _ := events[i].Metadata[circuit.ExtraKeyTraceID].(string)
			if traceID == "" {
				// No trace_id in the event metadata — can't look up child.
				continue
			}
			if childDir, ok := childByTraceID[traceID]; ok {
				childPath := filepath.Join(childDir, "trace.jsonl")
				childEvents, err := readTraceEvents(childPath)
				if err == nil {
					for j := range childEvents {
						ce := annotateEvent(&childEvents[j], fmt.Sprintf("  [%s]", label))
						out = append(out, ce)
					}
				}
			}

		case "delegate_end":
			label := ct
			if label == "" {
				label = labelUnknown
			}
			ev := annotateEvent(&events[i], fmt.Sprintf("[DELEGATION END: %s]", label))
			out = append(out, ev)

		default:
			out = append(out, events[i])
		}
	}
	return out
}

// annotateEvent prepends a marker to an event's Event field.
func annotateEvent(ev *engine.TraceEvent, marker string) engine.TraceEvent {
	ev.Event = marker + " " + ev.Event
	return *ev
}

// indexChildRuns scans {stateDir}/runs/*/run.json and returns a map from
// trace_id to run directory path. Only runs with a non-empty TraceID are indexed.
func indexChildRuns(stateDir string) map[string]string {
	result := make(map[string]string)
	if stateDir == "" {
		return result
	}
	runsDir := filepath.Join(stateDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runDir := filepath.Join(runsDir, e.Name())
		rec, err := engine.LoadRunRecord(runDir)
		if err != nil || rec.TraceID == "" {
			continue
		}
		result[rec.TraceID] = runDir
	}
	return result
}

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	framework "github.com/dpopsuev/origami"
)

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
		sd = ".origami/state"
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
	maxLevel := framework.LevelInfo
	if *v {
		maxLevel = framework.LevelDebug
	}
	if *vv {
		maxLevel = framework.LevelTrace
	}

	filtered := filterEvents(events, maxLevel, *caseFilter, *nodeFilter, *errorsOnly)

	if *follow {
		filtered = annotateDelegations(filtered, sd)
	}

	switch *format {
	case "json":
		return renderTraceJSON(w, filtered)
	case "text":
		return renderTraceText(w, filtered)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

func resolveTracePath(stateDir, runID string) (string, error) {
	if stateDir == "" {
		stateDir = os.Getenv("ORIGAMI_STATE_DIR")
	}
	if stateDir == "" {
		stateDir = ".origami/state"
	}

	runsDir := filepath.Join(stateDir, "runs")

	if runID == "" {
		// Find most recent run by mtime.
		entries, err := os.ReadDir(runsDir)
		if err != nil {
			return "", fmt.Errorf("cannot read runs directory %s: %w", runsDir, err)
		}
		var dirs []os.DirEntry
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e)
			}
		}
		if len(dirs) == 0 {
			return "", fmt.Errorf("no runs found in %s", runsDir)
		}
		sort.Slice(dirs, func(i, j int) bool {
			fi, _ := dirs[i].Info()
			fj, _ := dirs[j].Info()
			return fi.ModTime().After(fj.ModTime())
		})
		runID = dirs[0].Name()
	}

	return filepath.Join(runsDir, runID, "trace.jsonl"), nil
}

func readTraceEvents(path string) ([]framework.TraceEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}
	defer f.Close()

	var events []framework.TraceEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev framework.TraceEvent
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

func traceLevelRank(l framework.TraceLevel) int {
	switch l {
	case framework.LevelInfo:
		return 0
	case framework.LevelDebug:
		return 1
	case framework.LevelTrace:
		return 2
	default:
		return 3
	}
}

func filterEvents(events []framework.TraceEvent, maxLevel framework.TraceLevel, caseID, node string, errorsOnly bool) []framework.TraceEvent {
	maxRank := traceLevelRank(maxLevel)
	var out []framework.TraceEvent
	for _, ev := range events {
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
		out = append(out, ev)
	}
	return out
}

func renderTraceJSON(w io.Writer, events []framework.TraceEvent) error {
	enc := json.NewEncoder(w)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}
	return nil
}

func renderTraceText(w io.Writer, events []framework.TraceEvent) error {
	for _, ev := range events {
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
func annotateDelegations(events []framework.TraceEvent, stateDir string) []framework.TraceEvent {
	// Build a trace_id → run directory index for child trace lookup.
	childByTraceID := indexChildRuns(stateDir)

	var out []framework.TraceEvent
	for _, ev := range events {
		ct, _ := ev.Metadata[framework.ExtraKeyCircuitType].(string)
		switch ev.Event {
		case "delegate_start":
			label := ct
			if label == "" {
				label = "unknown"
			}
			ev = annotateEvent(ev, fmt.Sprintf("[DELEGATION START: %s]", label))
			out = append(out, ev)

			// Try to inline child trace events.
			traceID, _ := ev.Metadata[framework.ExtraKeyTraceID].(string)
			if traceID == "" {
				// No trace_id in the event metadata — can't look up child.
				continue
			}
			if childDir, ok := childByTraceID[traceID]; ok {
				childPath := filepath.Join(childDir, "trace.jsonl")
				childEvents, err := readTraceEvents(childPath)
				if err == nil {
					for _, ce := range childEvents {
						ce = annotateEvent(ce, fmt.Sprintf("  [%s]", label))
						out = append(out, ce)
					}
				}
			}

		case "delegate_end":
			label := ct
			if label == "" {
				label = "unknown"
			}
			ev = annotateEvent(ev, fmt.Sprintf("[DELEGATION END: %s]", label))
			out = append(out, ev)

		default:
			out = append(out, ev)
		}
	}
	return out
}

// annotateEvent prepends a marker to an event's Event field.
func annotateEvent(ev framework.TraceEvent, marker string) framework.TraceEvent {
	ev.Event = marker + " " + ev.Event
	return ev
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
		rec, err := framework.LoadRunRecord(runDir)
		if err != nil || rec.TraceID == "" {
			continue
		}
		result[rec.TraceID] = runDir
	}
	return result
}

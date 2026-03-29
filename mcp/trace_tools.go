package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const traceLabelUnknown = "unknown"

// --- MCP tool input/output types for trace tools ---

// traceInput is the unified input for the consolidated "trace" tool.
type traceInput struct {
	Action            string `json:"action"` // events, report, diff
	SessionID         string `json:"session_id,omitempty"`
	Level             string `json:"level,omitempty"`
	CaseID            string `json:"case_id,omitempty"`
	Node              string `json:"node,omitempty"`
	Since             int    `json:"since,omitempty"`
	Limit             int    `json:"limit,omitempty"`
	FollowDelegations bool   `json:"follow_delegations,omitempty"`
	RunIDA            string `json:"run_id_a,omitempty"`
	RunIDB            string `json:"run_id_b,omitempty"`
}

type getTraceInput struct {
	SessionID         string `json:"session_id" jsonschema:"session ID from start_circuit"`
	Level             string `json:"level,omitempty" jsonschema:"filter: info, debug, or trace (default: info)"`
	CaseID            string `json:"case_id,omitempty" jsonschema:"filter by case ID"`
	Node              string `json:"node,omitempty" jsonschema:"filter by node name"`
	Since             int    `json:"since,omitempty" jsonschema:"return events from this index onward"`
	Limit             int    `json:"limit,omitempty" jsonschema:"max events to return (default: 500)"`
	FollowDelegations bool   `json:"follow_delegations,omitempty" jsonschema:"when true, annotate delegation events and inline child traces from the same StateDir"`
}

type getTraceOutput struct {
	Events []engine.TraceEvent `json:"events"`
	Total  int                 `json:"total"`
}

type getRunReportInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
}

type diffRunsInput struct {
	RunIDA string `json:"run_id_a" jsonschema:"first run ID"`
	RunIDB string `json:"run_id_b" jsonschema:"second run ID"`
}

type metricDelta struct {
	ID     string  `json:"id"`
	Name   string  `json:"name,omitempty"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
	Delta  float64 `json:"delta"`
}

type diffRunsOutput struct {
	MetricDeltas []metricDelta `json:"metric_deltas"`
}

// registerTraceTools adds the consolidated "trace" MCP tool.
// Only called when StateDir is configured.
func (s *CircuitServer) registerTraceTools() {
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "trace",
		Description: "Execution trace and run comparison. Actions: events (trace events with filters), report (persisted run report), diff (compare two runs).",
	}, NoOutputSchema(s.handleTraceDispatch))
}

// handleTraceDispatch routes the consolidated trace tool to the appropriate handler.
func (s *CircuitServer) handleTraceDispatch(ctx context.Context, req *sdkmcp.CallToolRequest, input *traceInput) (*sdkmcp.CallToolResult, any, error) {
	switch input.Action {
	case "events":
		eventsInput := getTraceInput{
			SessionID:         input.SessionID,
			Level:             input.Level,
			CaseID:            input.CaseID,
			Node:              input.Node,
			Since:             input.Since,
			Limit:             input.Limit,
			FollowDelegations: input.FollowDelegations,
		}
		res, out, err := s.handleGetTrace(ctx, req, &eventsInput)
		return res, out, err

	case "report":
		reportInput := getRunReportInput{SessionID: input.SessionID}
		return s.handleGetRunReport(ctx, req, reportInput)

	case "diff":
		diffInput := diffRunsInput{
			RunIDA: input.RunIDA,
			RunIDB: input.RunIDB,
		}
		res, out, err := s.handleDiffRuns(ctx, req, diffInput)
		return res, out, err

	default:
		return nil, nil, fmt.Errorf("%w: %q; valid actions: events, report, diff", ErrUnknownTraceAction, input.Action)
	}
}

func (s *CircuitServer) handleGetTrace(_ context.Context, _ *sdkmcp.CallToolRequest, input *getTraceInput) (*sdkmcp.CallToolResult, getTraceOutput, error) {
	runDir := s.resolveRunDir(input.SessionID)
	if runDir == "" {
		return nil, getTraceOutput{}, ErrNoTraceDataStateDirNotConfiguredOrRunNotFound
	}

	tracePath := filepath.Join(runDir, "trace.jsonl")
	f, err := os.Open(tracePath)
	if err != nil {
		return nil, getTraceOutput{}, fmt.Errorf("open trace: %w", err)
	}
	defer f.Close()

	maxLevel := engine.LevelInfo
	switch strings.ToLower(input.Level) {
	case "debug":
		maxLevel = engine.LevelDebug
	case "trace":
		maxLevel = engine.LevelTrace
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 500
	}

	var allEvents []engine.TraceEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var te engine.TraceEvent
		if err := json.Unmarshal(scanner.Bytes(), &te); err != nil {
			continue
		}
		if !levelIncludes(maxLevel, te.Level) {
			continue
		}
		if input.CaseID != "" && te.CaseID != input.CaseID {
			continue
		}
		if input.Node != "" && te.Node != input.Node {
			continue
		}
		allEvents = append(allEvents, te)
	}

	// When follow_delegations is set, annotate delegation events and
	// inline child traces found in the same StateDir.
	if input.FollowDelegations {
		allEvents = mergeChildTraces(allEvents, s.Config.StateDir, s.Config.GatewayEndpoint)
	}

	// Apply pagination.
	total := len(allEvents)
	events := make([]engine.TraceEvent, 0, limit)
	for i := range allEvents {
		if i < input.Since {
			continue
		}
		if len(events) >= limit {
			break
		}
		events = append(events, allEvents[i])
	}

	return nil, getTraceOutput{Events: events, Total: total}, nil
}

func (s *CircuitServer) handleGetRunReport(_ context.Context, _ *sdkmcp.CallToolRequest, input getRunReportInput) (*sdkmcp.CallToolResult, any, error) {
	runDir := s.resolveRunDir(input.SessionID)
	if runDir == "" {
		return nil, nil, ErrNoReportDataStateDirNotConfiguredOrRunNotFound
	}

	reportPath := filepath.Join(runDir, "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read report: %w", err)
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("parse report: %w", err)
	}
	return nil, result, nil
}

func (s *CircuitServer) handleDiffRuns(_ context.Context, _ *sdkmcp.CallToolRequest, input diffRunsInput) (*sdkmcp.CallToolResult, diffRunsOutput, error) { //nolint:unparam // handler pattern
	stateDir := s.Config.StateDir
	if stateDir == "" {
		return nil, diffRunsOutput{}, ErrStateDirNotConfigured
	}

	loadMetrics := func(runID string) ([]metricEntry, error) {
		path := filepath.Join(stateDir, "runs", runID, "report.json")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", runID, err)
		}
		var report struct {
			Metrics struct {
				Metrics []metricEntry `json:"metrics"`
			} `json:"metrics"`
		}
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("parse %s: %w", runID, err)
		}
		return report.Metrics.Metrics, nil
	}

	metricsA, err := loadMetrics(input.RunIDA)
	if err != nil {
		return nil, diffRunsOutput{}, err
	}
	metricsB, err := loadMetrics(input.RunIDB)
	if err != nil {
		return nil, diffRunsOutput{}, err
	}

	byID := make(map[string]metricEntry, len(metricsA))
	for _, m := range metricsA {
		byID[m.ID] = m
	}

	deltas := make([]metricDelta, 0, len(metricsB))
	for _, mb := range metricsB {
		ma := byID[mb.ID]
		deltas = append(deltas, metricDelta{
			ID:     mb.ID,
			Name:   mb.Name,
			Before: ma.Score,
			After:  mb.Score,
			Delta:  mb.Score - ma.Score,
		})
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].ID < deltas[j].ID })

	return nil, diffRunsOutput{MetricDeltas: deltas}, nil
}

type metricEntry struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// resolveRunDir returns the run directory for the given session ID.
func (s *CircuitServer) resolveRunDir(sessionID string) string {
	stateDir := s.Config.StateDir
	if stateDir == "" {
		return ""
	}
	dir := filepath.Join(stateDir, "runs", sessionID)
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

// levelIncludes returns true if the event level is within the max level.
func levelIncludes(maxLevel, event engine.TraceLevel) bool {
	order := map[engine.TraceLevel]int{
		engine.LevelInfo:  0,
		engine.LevelDebug: 1,
		engine.LevelTrace: 2,
	}
	return order[event] <= order[maxLevel]
}

// mergeChildTraces annotates delegate_start/delegate_end events and inlines
// child trace events from the same StateDir when a matching trace_id is found.
// Child events are annotated with source metadata for identification.
//
// When mediatorEndpoint is set and a child trace is not found locally,
// mergeChildTraces attempts to fetch it via the mediator's get_trace tool.
func mergeChildTraces(events []engine.TraceEvent, stateDir, mediatorEndpoint string) []engine.TraceEvent {
	childByTraceID := indexRunsByTraceID(stateDir)

	var out []engine.TraceEvent
	for i := range events {
		ct, _ := events[i].Metadata[circuit.ExtraKeyCircuitType].(string)
		switch events[i].Event {
		case "delegate_start":
			label := ct
			if label == "" {
				label = traceLabelUnknown
			}
			ev := events[i]
			if ev.Metadata == nil {
				ev.Metadata = make(map[string]any)
			}
			ev.Metadata[circuit.TraceMetaDelegation] = label

			out = append(out, ev)

			// Try to inline child trace events.
			traceID, _ := ev.Metadata[circuit.ExtraKeyTraceID].(string)
			if traceID == "" {
				continue
			}
			if childDir, ok := childByTraceID[traceID]; ok {
				// Local child trace found.
				childPath := filepath.Join(childDir, "trace.jsonl")
				childEvents := readTraceFile(childPath)
				for j := range childEvents {
					if childEvents[j].Metadata == nil {
						childEvents[j].Metadata = make(map[string]any)
					}
					childEvents[j].Metadata[circuit.TraceMetaSource] = label
					out = append(out, childEvents[j])
				}
			} else if mediatorEndpoint != "" {
				// Cross-service: fetch child trace via mediator.
				childEvents := fetchRemoteTrace(mediatorEndpoint, traceID)
				for j := range childEvents {
					if childEvents[j].Metadata == nil {
						childEvents[j].Metadata = make(map[string]any)
					}
					childEvents[j].Metadata[circuit.TraceMetaSource] = label + " (remote)"
					out = append(out, childEvents[j])
				}
			}

		case "delegate_end":
			label := ct
			if label == "" {
				label = traceLabelUnknown
			}
			ev := events[i]
			if ev.Metadata == nil {
				ev.Metadata = make(map[string]any)
			}
			ev.Metadata[circuit.TraceMetaDelegation] = label
			out = append(out, ev)

		default:
			out = append(out, events[i])
		}
	}

	// Sort by timestamp for interleaved child events.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp < out[j].Timestamp
	})

	return out
}

// indexRunsByTraceID scans {stateDir}/runs/*/run.json and returns a map
// from trace_id to run directory. Only runs with a non-empty TraceID are indexed.
func indexRunsByTraceID(stateDir string) map[string]string {
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

// fetchRemoteTrace calls get_trace on a remote MCP endpoint to fetch child
// trace events for cross-service trace merging. Returns nil on any error
// (best-effort — remote service may be offline).
func fetchRemoteTrace(endpoint, traceID string) []engine.TraceEvent {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := &sdkmcp.StreamableClientTransport{Endpoint: endpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-trace-merge", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "get_trace",
		Arguments: mustMarshalJSON(map[string]any{
			circuit.ProtoKeySessionID: traceID,
			"follow_delegations":      true,
			"level":                   "info",
			"limit":                   1000,
		}),
	})
	if err != nil || result.IsError {
		return nil
	}

	for _, c := range result.Content {
		tc, ok := c.(*sdkmcp.TextContent)
		if !ok {
			continue
		}
		var out getTraceOutput
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			continue
		}
		return out.Events
	}
	return nil
}

func mustMarshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// readTraceFile reads all trace events from a JSONL file. Returns nil on error.
func readTraceFile(path string) []engine.TraceEvent {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var events []engine.TraceEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		var ev engine.TraceEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events
}

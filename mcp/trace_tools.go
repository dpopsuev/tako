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

	framework "github.com/dpopsuev/origami"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- MCP tool input/output types for trace tools ---

type getTraceInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
	Level     string `json:"level,omitempty" jsonschema:"filter: info, debug, or trace (default: info)"`
	CaseID    string `json:"case_id,omitempty" jsonschema:"filter by case ID"`
	Node      string `json:"node,omitempty" jsonschema:"filter by node name"`
	Since     int    `json:"since,omitempty" jsonschema:"return events from this index onward"`
	Limit     int    `json:"limit,omitempty" jsonschema:"max events to return (default: 500)"`
}

type getTraceOutput struct {
	Events []framework.TraceEvent `json:"events"`
	Total  int                    `json:"total"`
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

// registerTraceTools adds get_trace, get_run_report, and diff_runs MCP tools.
// Only called when StateDir is configured.
func (s *CircuitServer) registerTraceTools() {
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_trace",
		Description: "Read execution trace events from a completed run. Filter by level (info/debug/trace), case ID, and node name. Returns structured JSON — no glue code needed.",
	}, NoOutputSchema(s.handleGetTrace))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_run_report",
		Description: "Get the structured calibration report for a completed run. Returns metrics, per-case results, and aggregate accuracy as JSON.",
	}, NoOutputSchema(s.handleGetRunReport))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "diff_runs",
		Description: "Compare two calibration runs. Returns per-metric deltas showing regressions and improvements.",
	}, NoOutputSchema(s.handleDiffRuns))
}

func (s *CircuitServer) handleGetTrace(_ context.Context, _ *sdkmcp.CallToolRequest, input getTraceInput) (*sdkmcp.CallToolResult, getTraceOutput, error) {
	runDir := s.resolveRunDir(input.SessionID)
	if runDir == "" {
		return nil, getTraceOutput{}, fmt.Errorf("no trace data: StateDir not configured or run not found")
	}

	tracePath := filepath.Join(runDir, "trace.jsonl")
	f, err := os.Open(tracePath)
	if err != nil {
		return nil, getTraceOutput{}, fmt.Errorf("open trace: %w", err)
	}
	defer f.Close()

	maxLevel := framework.LevelInfo
	switch strings.ToLower(input.Level) {
	case "debug":
		maxLevel = framework.LevelDebug
	case "trace":
		maxLevel = framework.LevelTrace
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 500
	}

	var events []framework.TraceEvent
	total := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var te framework.TraceEvent
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
		total++
		if total <= input.Since {
			continue
		}
		if len(events) < limit {
			events = append(events, te)
		}
	}

	return nil, getTraceOutput{Events: events, Total: total}, nil
}

func (s *CircuitServer) handleGetRunReport(_ context.Context, _ *sdkmcp.CallToolRequest, input getRunReportInput) (*sdkmcp.CallToolResult, any, error) {
	runDir := s.resolveRunDir(input.SessionID)
	if runDir == "" {
		return nil, nil, fmt.Errorf("no report data: StateDir not configured or run not found")
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

func (s *CircuitServer) handleDiffRuns(_ context.Context, _ *sdkmcp.CallToolRequest, input diffRunsInput) (*sdkmcp.CallToolResult, diffRunsOutput, error) {
	stateDir := s.Config.StateDir
	if stateDir == "" {
		return nil, diffRunsOutput{}, fmt.Errorf("StateDir not configured")
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

	var deltas []metricDelta
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
func levelIncludes(max, event framework.TraceLevel) bool {
	order := map[framework.TraceLevel]int{
		framework.LevelInfo:  0,
		framework.LevelDebug: 1,
		framework.LevelTrace: 2,
	}
	return order[event] <= order[max]
}

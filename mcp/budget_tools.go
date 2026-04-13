package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// budgetInput is the unified input for the "budget" tool.
type budgetInput struct {
	Action    string `json:"action" jsonschema:"required,enum=summary"`
	SessionID string `json:"session_id,omitempty" jsonschema:"session ID"`
}

type budgetOutput struct {
	SessionID   string `json:"session_id,omitempty"`
	Alias       string `json:"alias,omitempty"`
	TotalCases  int    `json:"total_cases"`
	WorkerCount int    `json:"worker_count"`
	SignalCount int    `json:"signal_count"`
	Status      string `json:"status"`
}

// registerBudgetTool adds the "budget" MCP tool.
func (s *CircuitServer) registerBudgetTool() {
	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "budget",
		Description: "Circuit execution budget and progress. Actions: summary (session/worker/signal summary).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleBudgetDispatch))
}

func (s *CircuitServer) handleBudgetDispatch(_ context.Context, _ *sdkmcp.CallToolRequest, input budgetInput) (*sdkmcp.CallToolResult, any, error) {
	switch input.Action {
	case "summary":
		return s.handleBudgetSummary(input)
	default:
		return toolError(fmt.Errorf("%w: %q; valid: summary", ErrUnknownBudgetAction, input.Action)), nil, nil
	}
}

func (s *CircuitServer) handleBudgetSummary(input budgetInput) (*sdkmcp.CallToolResult, any, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()

	if sess == nil {
		return toolError(ErrNoActiveSession), nil, nil
	}

	if input.SessionID != "" && sess.ID != input.SessionID && sess.Alias != input.SessionID {
		return toolError(fmt.Errorf("%w: %q", ErrSessionId, input.SessionID)), nil, nil
	}

	sess.mu.Lock()
	workerCount := len(sess.registeredWorkers)
	sess.mu.Unlock()

	out := budgetOutput{
		SessionID:   sess.ID,
		Alias:       sess.Alias,
		TotalCases:  sess.TotalCases,
		WorkerCount: workerCount,
		SignalCount: sess.Bus.Len(),
		Status:      string(sess.state),
	}

	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}

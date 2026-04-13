package mcp

import (
	"encoding/json"
	"fmt"

	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// operatorInput is the unified input for the "operator" tool.
type operatorInput struct {
	Action    string `json:"action" jsonschema:"required,enum=observe;status"`
	SessionID string `json:"session_id,omitempty" jsonschema:"session ID for context"`
}

type operatorObserveOutput struct {
	SessionID   string `json:"session_id,omitempty"`
	Alias       string `json:"alias,omitempty"`
	Status      string `json:"status"`
	WorkerCount int    `json:"worker_count"`
	SignalCount int    `json:"signal_count"`
}

type operatorStatusOutput struct {
	HasSession bool   `json:"has_session"`
	Status     string `json:"status"`
}

// registerOperatorTool adds the "operator" MCP tool.
func (s *CircuitServer) registerOperatorTool() {
	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "operator",
		Description: "Operator reconciliation loop. Actions: observe (current system state), status (operator availability).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleOperatorDispatch))
}

func (s *CircuitServer) handleOperatorDispatch(_ context.Context, _ *sdkmcp.CallToolRequest, input operatorInput) (*sdkmcp.CallToolResult, any, error) {
	switch input.Action {
	case "status":
		return s.handleOperatorStatus()
	case "observe":
		return s.handleOperatorObserve()
	default:
		return toolError(fmt.Errorf("%w: %q; valid actions: observe, status", ErrUnknownOperatorAction, input.Action)), nil, nil
	}
}

func (s *CircuitServer) handleOperatorStatus() (*sdkmcp.CallToolResult, any, error) {
	s.mu.Lock()
	hasSession := s.session != nil
	s.mu.Unlock()

	status := "idle"
	if hasSession {
		status = "active"
	}

	out := operatorStatusOutput{HasSession: hasSession, Status: status}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}

func (s *CircuitServer) handleOperatorObserve() (*sdkmcp.CallToolResult, any, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()

	if sess == nil {
		out := operatorObserveOutput{Status: "idle"}
		data, _ := json.Marshal(out)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
		}, out, nil
	}

	sess.mu.Lock()
	workerCount := len(sess.registeredWorkers)
	sess.mu.Unlock()

	out := operatorObserveOutput{
		SessionID:   sess.ID,
		Alias:       sess.Alias,
		Status:      "active",
		WorkerCount: workerCount,
		SignalCount: sess.Bus.Len(),
	}

	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}

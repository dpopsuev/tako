package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

// mcpCircuitTransformer implements Transformer by delegating to a remote
// schematic via the Papercup protocol through the Origami Mediator.
// When a circuit node references a sub-circuit that isn't available locally,
// this transformer drives start_circuit → get_next_step (poll) → get_report
// against the mediator endpoint.
type mcpCircuitTransformer struct {
	circuitType string // handler name from NodeDef (e.g., "dsr")
	endpoint    string // mediator MCP URL
}

func (t *mcpCircuitTransformer) Name() string { return "mediator.circuit" }

func (t *mcpCircuitTransformer) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	slog.Debug("mediator delegate start",
		"circuit_type", t.circuitType,
		"endpoint", t.endpoint,
		"node", tc.NodeName,
	)

	transport := &sdkmcp.StreamableClientTransport{Endpoint: t.endpoint}
	connector := subprocess.DefaultConnector()
	session, err := connector.Connect(ctx, transport)
	if err != nil {
		return nil, fmt.Errorf("mediator connect to %s for circuit_type %q: %w",
			t.endpoint, t.circuitType, err)
	}
	defer session.Close()

	// Build extra params: circuit_type + forwarded walker context.
	extra := map[string]any{"circuit_type": t.circuitType}
	if tc.WalkerState != nil {
		for k, v := range tc.WalkerState.Context {
			extra[k] = v
		}
	}

	// start_circuit
	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "start_circuit",
		Arguments: mustMarshal(map[string]any{"extra": extra}),
	})
	if err != nil {
		return nil, fmt.Errorf("mediator start_circuit(%s): %w", t.circuitType, err)
	}
	startOut, err := parseToolResult(startResult)
	if err != nil {
		return nil, fmt.Errorf("mediator start_circuit(%s) parse: %w", t.circuitType, err)
	}
	sessionID, _ := startOut["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("mediator start_circuit(%s): no session_id in response", t.circuitType)
	}

	slog.Debug("mediator delegate session started",
		"circuit_type", t.circuitType,
		"session_id", sessionID,
	)

	// Poll get_next_step until done.
	for {
		stepResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "get_next_step",
			Arguments: mustMarshal(map[string]any{
				"session_id": sessionID,
				"timeout_ms": 30000,
			}),
		})
		if err != nil {
			return nil, fmt.Errorf("mediator get_next_step(%s): %w", t.circuitType, err)
		}
		stepOut, err := parseToolResult(stepResult)
		if err != nil {
			return nil, fmt.Errorf("mediator get_next_step(%s) parse: %w", t.circuitType, err)
		}

		if done, _ := stepOut["done"].(bool); done {
			if errMsg, ok := stepOut["error"].(string); ok && errMsg != "" {
				return nil, fmt.Errorf("mediator circuit %q failed: %s", t.circuitType, errMsg)
			}
			break
		}
	}

	// get_report
	reportResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "get_report",
		Arguments: mustMarshal(map[string]any{"session_id": sessionID}),
	})
	if err != nil {
		return nil, fmt.Errorf("mediator get_report(%s): %w", t.circuitType, err)
	}
	reportOut, err := parseToolResult(reportResult)
	if err != nil {
		return nil, fmt.Errorf("mediator get_report(%s) parse: %w", t.circuitType, err)
	}

	if errMsg, ok := reportOut["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("mediator circuit %q error: %s", t.circuitType, errMsg)
	}

	slog.Debug("mediator delegate complete",
		"circuit_type", t.circuitType,
		"session_id", sessionID,
		"status", reportOut["status"],
	)

	// Return the structured report as the node's artifact.
	if structured, ok := reportOut["structured"]; ok {
		return structured, nil
	}
	return reportOut, nil
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func parseToolResult(result *sdkmcp.CallToolResult) (map[string]any, error) {
	if result == nil {
		return nil, fmt.Errorf("nil result")
	}
	if result.IsError {
		for _, c := range result.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return nil, fmt.Errorf("%s", tc.Text)
			}
		}
		return nil, fmt.Errorf("tool returned error")
	}
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			var out map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
				return nil, fmt.Errorf("unmarshal: %w", err)
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("no text content in result")
}

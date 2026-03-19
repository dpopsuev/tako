package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

// PromptRelayer dispatches a prompt and blocks until an artifact is returned.
// This bridges the integrated circuit's dispatcher with a subgraph circuit's
// delegation. The dispatch package's MuxDispatcher implements this.
type PromptRelayer interface {
	Dispatch(ctx context.Context, dc PromptRelayContext) ([]byte, error)
}

// PromptRelayContext carries the prompt data for relay dispatch.
type PromptRelayContext struct {
	CaseID        string
	Step          string
	PromptContent string
	ArtifactPath  string
}

// ContextKeyPromptRelayer is the walker context key for the integrated circuit's
// dispatcher. Set automatically by calibrate.Run when PromptRelayer is provided.
const ContextKeyPromptRelayer = "_prompt_relayer"

// mcpCircuitTransformer implements Transformer by delegating to a remote
// schematic via the Papercup protocol through the Origami Mediator.
// When a circuit node references a sub-circuit that isn't available locally,
// this transformer drives start_circuit → get_next_step → submit_step → get_report
// against the mediator endpoint. Subgraph prompts are relayed through the
// integrated circuit's dispatcher so the same agent workers process them.
type mcpCircuitTransformer struct {
	circuitType string // handler name from NodeDef (e.g., "gnd")
	endpoint    string // mediator MCP URL
}

func (t *mcpCircuitTransformer) Name() string { return "mediator.circuit" }

func (t *mcpCircuitTransformer) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	slog.Debug("mediator delegate start",
		"circuit_type", t.circuitType,
		"endpoint", t.endpoint,
		"node", tc.NodeName,
	)

	// Get integrated circuit's dispatcher from walker context for prompt relay.
	var relayer PromptRelayer
	if tc.WalkerState != nil {
		relayer, _ = tc.WalkerState.Context[ContextKeyPromptRelayer].(PromptRelayer)
	}

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
			if k == ContextKeyPromptRelayer {
				continue // don't serialize the relayer
			}
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
		"has_relayer", relayer != nil,
	)

	// Drive child circuit: get_next_step → relay prompt → submit_step until done.
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

		available, _ := stepOut["available"].(bool)
		if !available {
			continue
		}

		// Subgraph has a prompt — relay through integrated circuit's dispatcher.
		childStep, _ := stepOut["step"].(string)
		childDispatchID, _ := stepOut["dispatch_id"].(float64)
		childPrompt, _ := stepOut["prompt_content"].(string)
		childCaseID, _ := stepOut["case_id"].(string)
		childArtifactPath, _ := stepOut["artifact_path"].(string)

		slog.Debug("mediator relay child prompt",
			"circuit_type", t.circuitType,
			"child_step", childStep,
			"child_case_id", childCaseID,
			"child_dispatch_id", int64(childDispatchID),
		)

		var artifactData []byte
		if relayer != nil {
			// Relay through integrated circuit's dispatcher — agent workers process it.
			artifactData, err = relayer.Dispatch(ctx, PromptRelayContext{
				CaseID:        childCaseID,
				Step:          "delegate:" + t.circuitType + ":" + childStep,
				PromptContent: childPrompt,
				ArtifactPath:  childArtifactPath,
			})
			if err != nil {
				return nil, fmt.Errorf("mediator relay dispatch(%s/%s): %w", t.circuitType, childStep, err)
			}
		} else {
			// No relayer — cannot process child prompts.
			return nil, fmt.Errorf("mediator circuit %q dispatched prompt for step %q but no PromptRelayer configured (set ContextKeyPromptRelayer in walker context)", t.circuitType, childStep)
		}

		// Parse artifact and submit back to child.
		var fields map[string]any
		if err := json.Unmarshal(artifactData, &fields); err != nil {
			// If not JSON, wrap as raw content.
			fields = map[string]any{"content": string(artifactData)}
		}

		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "submit_step",
			Arguments: mustMarshal(map[string]any{
				"session_id":  sessionID,
				"dispatch_id": int64(childDispatchID),
				"step":        childStep,
				"fields":      fields,
			}),
		})
		if err != nil {
			return nil, fmt.Errorf("mediator submit_step(%s/%s): %w", t.circuitType, childStep, err)
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

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/subprocess"
)

// PromptRelayer is a type alias for circuit.PromptRelayer.
type PromptRelayer = circuit.PromptRelayer

// PromptRelayContext is a type alias for circuit.PromptRelayContext.
type PromptRelayContext = circuit.PromptRelayContext

// ContextKeyPromptRelayer is the walker context key for the integrated circuit's
// dispatcher.
const ContextKeyPromptRelayer = circuit.ContextKeyPromptRelayer

// MCPCircuitTransformer implements Transformer by delegating to a remote
// schematic via the Papercup protocol through the Origami Mediator.
// Exported for test backward compatibility; canonical usage is through
// BuildGraph with instrument: circuit.
type MCPCircuitTransformer struct {
	CircuitType string // handler name from circuit.NodeDef (e.g., "beta")
	Endpoint    string // mediator MCP URL
}

func (t *MCPCircuitTransformer) Name() string { return "mediator.circuit" }

//nolint:gocyclo,funlen // MCP client loop with session lifecycle — complexity is inherent
func (t *MCPCircuitTransformer) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	slog.DebugContext(ctx, circuit.LogMediatorDelegateStart, slog.Any(circuit.LogKeyCircuitType, t.CircuitType), slog.Any(circuit.LogKeyEndpoint, t.Endpoint), slog.Any(circuit.LogKeyNode, tc.NodeName))

	var relayer PromptRelayer
	if tc.WalkerState != nil {
		relayer, _ = tc.WalkerState.Context[ContextKeyPromptRelayer].(PromptRelayer)
	}

	transport := &sdkmcp.StreamableClientTransport{Endpoint: t.Endpoint}
	connector := subprocess.DefaultConnector()
	session, err := connector.Connect(ctx, transport)
	if err != nil {
		return nil, fmt.Errorf("mediator connect to %s for circuit_type %q: %w",
			t.Endpoint, t.CircuitType, err)
	}
	defer session.Close()

	extra := map[string]any{circuit.ExtraKeyCircuitType: t.CircuitType}
	if tc.WalkerState != nil {
		for k, v := range tc.WalkerState.Context {
			if k == ContextKeyPromptRelayer {
				continue
			}
			extra[k] = v
		}
	}

	if traceID, ok := tc.WalkerState.Context[circuit.ContextKeyTraceID].(string); ok {
		extra[circuit.ExtraKeyTraceID] = traceID
	} else {
		extra[circuit.ExtraKeyTraceID] = fmt.Sprintf("tr-%d", time.Now().UnixMilli())
	}

	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustMarshal(map[string]any{"action": "start", "extra": extra}),
	})
	if err != nil {
		return nil, fmt.Errorf("mediator circuit/start(%s): %w", t.CircuitType, err)
	}
	startOut, err := parseToolResult(startResult)
	if err != nil {
		return nil, fmt.Errorf("mediator circuit/start(%s) parse: %w", t.CircuitType, err)
	}
	sessionID, _ := startOut["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: (%s): no session_id in response", ErrMediatorCircuitStart, t.CircuitType)
	}

	slog.DebugContext(ctx, circuit.LogMediatorSessionStarted, slog.Any(circuit.LogKeyCircuitType, t.CircuitType), slog.Any(circuit.LogKeySessionID, sessionID), slog.Any(circuit.LogKeyHasRelayer, relayer != nil))

	for {
		stepResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshal(map[string]any{
				"action":     "step",
				"session_id": sessionID,
				"timeout_ms": 30000,
			}),
		})
		if err != nil {
			return nil, fmt.Errorf("mediator circuit/step(%s): %w", t.CircuitType, err)
		}
		stepOut, err := parseToolResult(stepResult)
		if err != nil {
			return nil, fmt.Errorf("mediator circuit/step(%s) parse: %w", t.CircuitType, err)
		}

		if done, _ := stepOut[circuit.ProtoKeyDone].(bool); done {
			if errMsg, ok := stepOut[circuit.ProtoKeyError].(string); ok && errMsg != "" {
				return nil, fmt.Errorf("%w: %q failed: %s", ErrMediatorCircuit, t.CircuitType, errMsg)
			}
			break
		}

		available, _ := stepOut[circuit.ProtoKeyAvailable].(bool)
		if !available {
			continue
		}

		childStep, _ := stepOut[circuit.ProtoKeyStep].(string)
		childDispatchID, _ := stepOut[circuit.ProtoKeyDispatchID].(float64)
		childPrompt, _ := stepOut[circuit.ProtoKeyPromptContent].(string)
		childCaseID, _ := stepOut[circuit.ProtoKeyCaseID].(string)
		childArtifactPath, _ := stepOut[circuit.ProtoKeyArtifactPath].(string)

		slog.DebugContext(ctx, circuit.LogMediatorRelayChild, slog.Any(circuit.LogKeyCircuitType, t.CircuitType), slog.Any(circuit.LogKeyChildStep, childStep), slog.Any(circuit.LogKeyChildCaseID, childCaseID), slog.Any(circuit.LogKeyChildDispatchID, int64(childDispatchID)))

		var artifactData []byte
		if relayer != nil {
			artifactData, err = relayer.Dispatch(ctx, PromptRelayContext{
				CaseID:        childCaseID,
				Step:          "delegate:" + t.CircuitType + ":" + childStep,
				PromptContent: childPrompt,
				ArtifactPath:  childArtifactPath,
			})
			if err != nil {
				return nil, fmt.Errorf("mediator relay dispatch(%s/%s): %w", t.CircuitType, childStep, err)
			}
		} else {
			return nil, fmt.Errorf("%w: %q dispatched prompt for step %q but no PromptRelayer configured (set ContextKeyPromptRelayer in walker context)", ErrMediatorCircuit, t.CircuitType, childStep)
		}

		var fields map[string]any
		if err := json.Unmarshal(artifactData, &fields); err != nil {
			fields = map[string]any{"content": string(artifactData)}
		}

		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshal(map[string]any{
				"action":                   "submit",
				circuit.ProtoKeySessionID:  sessionID,
				circuit.ProtoKeyDispatchID: int64(childDispatchID),
				circuit.ProtoKeyStep:       childStep,
				circuit.ProtoKeyFields:     fields,
			}),
		})
		if err != nil {
			return nil, fmt.Errorf("mediator circuit/submit(%s/%s): %w", t.CircuitType, childStep, err)
		}
	}

	reportResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustMarshal(map[string]any{"action": "report", circuit.ProtoKeySessionID: sessionID}),
	})
	if err != nil {
		return nil, fmt.Errorf("mediator circuit/report(%s): %w", t.CircuitType, err)
	}
	reportOut, err := parseToolResult(reportResult)
	if err != nil {
		return nil, fmt.Errorf("mediator circuit/report(%s) parse: %w", t.CircuitType, err)
	}

	if errMsg, ok := reportOut[circuit.ProtoKeyError].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%w: %q error: %s", ErrMediatorCircuit, t.CircuitType, errMsg)
	}

	slog.DebugContext(ctx, circuit.LogMediatorDelegateComplete, slog.Any(circuit.LogKeyCircuitType, t.CircuitType), slog.Any(circuit.LogKeySessionID, sessionID), slog.Any(circuit.LogKeyStatus, reportOut[circuit.ProtoKeyStatus]))

	if structured, ok := reportOut[circuit.ProtoKeyStructured]; ok {
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
		return nil, ErrNilResult
	}
	if result.IsError {
		for _, c := range result.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return nil, fmt.Errorf("%w: %s", ErrToolError, tc.Text)
			}
		}
		return nil, ErrToolReturnedError
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
	return nil, ErrNoTextContentInResult
}

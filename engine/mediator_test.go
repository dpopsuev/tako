package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newMockCircuitServer creates a mock MCP server that runs a stub circuit.
// The consolidated "circuit" tool dispatches on the "action" field:
// action=start completes immediately, action=step returns done=true,
// action=report returns the structured result.
func newMockCircuitServer(t *testing.T, result map[string]any, circuitErr string) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "mock-circuit", Version: "v0.1.0"},
		nil,
	)

	server.AddTool(
		&sdkmcp.Tool{
			Name:        "circuit",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var input struct {
				Action string `json:"action"`
			}
			if req.Params.Arguments != nil {
				json.Unmarshal(req.Params.Arguments, &input)
			}
			switch input.Action {
			case "start":
				out := map[string]any{
					"session_id":  "mock-session-1",
					"total_cases": 1,
					"status":      "running",
				}
				return mcpTextResult(out), nil
			case "step":
				out := map[string]any{"done": true}
				if circuitErr != "" {
					out["error"] = circuitErr
				}
				return mcpTextResult(out), nil
			case "report":
				out := map[string]any{
					"status":     "done",
					"structured": result,
				}
				if circuitErr != "" {
					out["status"] = "error"
					out["error"] = circuitErr
				}
				return mcpTextResult(out), nil
			default:
				return mcpTextResult(map[string]any{"error": "unknown action"}), nil
			}
		},
	)

	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func mcpTextResult(v any) *sdkmcp.CallToolResult {
	b, _ := json.Marshal(v)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(b)}},
	}
}

func TestMCPCircuitTransformer_StubBackend(t *testing.T) {
	wantResult := map[string]any{
		"cases_done": float64(3),
		"backend":    "dsr",
	}
	ts := newMockCircuitServer(t, wantResult, "")

	trans := &MCPCircuitTransformer{
		CircuitType: "dsr",
		Endpoint:    ts.URL + "/mcp",
	}

	tc := &InstrumentContext{
		NodeName:    "gather-code",
		WalkerState: circuit.NewWalkerState("test"),
	}
	tc.WalkerState.Context["scenario"] = "ptp"

	result, err := trans.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if resultMap["backend"] != "dsr" {
		t.Errorf("backend = %v, want dsr", resultMap["backend"])
	}
	if resultMap["cases_done"] != float64(3) {
		t.Errorf("cases_done = %v, want 3", resultMap["cases_done"])
	}
}

func TestMCPCircuitTransformer_ConnectionError(t *testing.T) {
	trans := &MCPCircuitTransformer{
		CircuitType: "dsr",
		Endpoint:    "http://127.0.0.1:1/mcp", // unreachable
	}

	tc := &InstrumentContext{
		NodeName:    "gather-code",
		WalkerState: circuit.NewWalkerState("test"),
	}

	_, err := trans.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
	if !strings.Contains(err.Error(), "mediator connect") {
		t.Errorf("error = %q, want to contain 'mediator connect'", err.Error())
	}
	if !strings.Contains(err.Error(), "dsr") {
		t.Errorf("error = %q, want to contain circuit_type 'dsr'", err.Error())
	}
}

func TestMCPCircuitTransformer_CircuitError(t *testing.T) {
	ts := newMockCircuitServer(t, nil, "all cases failed: node X broken")

	trans := &MCPCircuitTransformer{
		CircuitType: "dsr",
		Endpoint:    ts.URL + "/mcp",
	}

	tc := &InstrumentContext{
		NodeName:    "gather-code",
		WalkerState: circuit.NewWalkerState("test"),
	}

	_, err := trans.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error from failed circuit")
	}
	if !strings.Contains(err.Error(), "dsr") {
		t.Errorf("error = %q, want to contain circuit_type 'dsr'", err.Error())
	}
	if !strings.Contains(err.Error(), "all cases failed") {
		t.Errorf("error = %q, want to contain remote error message", err.Error())
	}
}

// --- TSK-186: trace_id propagation ---

// newCapturingCircuitServer creates a mock server that captures circuit start extra params.
func newCapturingCircuitServer(t *testing.T, captured map[string]any) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "capturing-circuit", Version: "v0.1.0"},
		nil,
	)

	server.AddTool(
		&sdkmcp.Tool{
			Name:        "circuit",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var input struct {
				Action string         `json:"action"`
				Extra  map[string]any `json:"extra"`
			}
			if req.Params.Arguments != nil {
				json.Unmarshal(req.Params.Arguments, &input)
			}
			switch input.Action {
			case "start":
				clear(captured)
				for k, v := range input.Extra {
					captured[k] = v
				}
				return mcpTextResult(map[string]any{
					"session_id":  "cap-session-1",
					"total_cases": 1,
					"status":      "running",
				}), nil
			case "step":
				return mcpTextResult(map[string]any{"done": true}), nil
			case "report":
				return mcpTextResult(map[string]any{
					"status":     "done",
					"structured": map[string]any{"ok": true},
				}), nil
			default:
				return mcpTextResult(map[string]any{"error": "unknown action"}), nil
			}
		},
	)

	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestMCPCircuitTransformer_PropagatesTraceID(t *testing.T) {
	captured := make(map[string]any)
	ts := newCapturingCircuitServer(t, captured)

	trans := &MCPCircuitTransformer{
		CircuitType: "beta",
		Endpoint:    ts.URL + "/mcp",
	}

	tc := &InstrumentContext{
		NodeName:    "gather-code",
		WalkerState: circuit.NewWalkerState("test"),
	}
	tc.WalkerState.Context["_trace_id"] = "tr-parent-42"

	_, err := trans.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// Verify trace_id was forwarded in extra params.
	if len(captured) == 0 {
		t.Fatal("captured extra is empty — circuit/start not called?")
	}
	traceID, ok := captured["trace_id"].(string)
	if !ok {
		t.Fatalf("trace_id not found in extra params; got: %v", captured)
	}
	if traceID != "tr-parent-42" {
		t.Errorf("trace_id = %q, want %q", traceID, "tr-parent-42")
	}
}

func TestMCPCircuitTransformer_GeneratesTraceIDWhenMissing(t *testing.T) {
	captured := make(map[string]any)
	ts := newCapturingCircuitServer(t, captured)

	trans := &MCPCircuitTransformer{
		CircuitType: "beta",
		Endpoint:    ts.URL + "/mcp",
	}

	tc := &InstrumentContext{
		NodeName:    "gather-code",
		WalkerState: circuit.NewWalkerState("test"),
	}
	// No _trace_id in context — should auto-generate.

	_, err := trans.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	traceID, ok := captured["trace_id"].(string)
	if !ok {
		t.Fatalf("trace_id not found in extra params; got: %v", captured)
	}
	if !strings.HasPrefix(traceID, "tr-") {
		t.Errorf("auto-generated trace_id = %q, expected prefix 'tr-'", traceID)
	}
}

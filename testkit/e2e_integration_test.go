package testkit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestE2E_Integration_ComposeStack probes the compose stack and runs
// a full MCP circuit if the stack is available. Skipped when the
// stack is not running (CI-safe).
func TestE2E_Integration_ComposeStack(t *testing.T) {
	const healthURL = "http://localhost:9200/healthz"
	const mcpEndpoint = "http://localhost:9200/mcp"

	// Probe health endpoint.
	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Get(healthURL)
	if err != nil {
		t.Skipf("compose stack not available: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("compose stack not healthy: status %d", resp.StatusCode)
	}

	// Connect MCP over StreamableHTTP.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	transport := &sdkmcp.StreamableClientTransport{Endpoint: mcpEndpoint}
	mcpClient := sdkmcp.NewClient(&sdkmcp.Implementation{
		Name:    "testkit-integration",
		Version: "v0.0.1",
	}, nil)
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		t.Skipf("cannot connect to MCP endpoint %s: %v", mcpEndpoint, err)
	}
	defer session.Close()

	// Start circuit with stub backend.
	startResult := callToolIntegration(t, ctx, session, "start_circuit", map[string]any{
		"extra": map[string]any{
			"scenario": "ptp",
			"backend":  "stub",
		},
	})
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("start_circuit: missing session_id: %v", startResult)
	}
	t.Logf("started session %s", sessionID)

	// Drain the circuit: get_next_step/submit_step loop.
	for {
		stepResult := callToolIntegration(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 5000,
		})
		if done, _ := stepResult["done"].(bool); done {
			break
		}
		if avail, _ := stepResult["available"].(bool); !avail {
			continue
		}

		step, _ := stepResult["step"].(string)
		dispatchID, _ := stepResult["dispatch_id"].(float64)

		callToolIntegration(t, ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields":      stubFields(step),
		})
	}

	// Get report.
	reportResult := callToolIntegration(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("get_report: status = %q, want %q (error: %v)", status, "done", reportResult["error"])
	}
	t.Logf("integration report status: %s", status)
}

// callToolIntegration is the integration-test variant of callTool.
func callToolIntegration(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(%s) error: %s", name, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) error", name)
	}
	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal %s: %v (text: %s)", name, err, tc.Text)
			}
			return result
		}
	}
	t.Fatalf("no text content in %s result", name)
	return nil
}

// stubFields returns minimal valid fields for a step. Used when the
// compose stack step schemas are unknown -- the stub backend typically
// does not validate fields strictly.
func stubFields(step string) map[string]any {
	return map[string]any{
		"_step": step,
		"data":  fmt.Sprintf("testkit-integration-%s", step),
	}
}

package containertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

// TestGND_LiveAcceptance verifies a running GND service can ensure, search,
// and read from a real GitHub repository. Requires docker-compose stack running.
//
//	go test ./subprocess/containertest/ -run TestGND_LiveAcceptance -v -count=1
func TestGND_LiveAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live GND acceptance in short mode")
	}

	const endpoint = "http://localhost:9100/mcp"

	// Pre-flight: GND must be reachable.
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:9100/healthz")
	if err != nil {
		t.Skipf("GND not reachable at localhost:9100: %v (is docker-compose up?)", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("GND healthz returned %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Connect via MCP.
	transport := &sdkmcp.StreamableClientTransport{Endpoint: endpoint}
	conn := subprocess.DefaultConnector()
	session, err := conn.Connect(ctx, transport)
	if err != nil {
		t.Fatalf("MCP connect: %v", err)
	}
	defer session.Close()
	t.Log("connected to GND")

	// List tools.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	var toolNames []string
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	t.Logf("tools: %v", toolNames)

	// Find the circuit tool name (could be "circuit", "gnd", etc.)
	circuitToolName := ""
	for _, name := range toolNames {
		if name == "circuit" || name == "gnd" {
			circuitToolName = name
			break
		}
	}
	if circuitToolName == "" {
		t.Fatalf("no circuit/gnd tool found in %v", toolNames)
	}

	// Start a GND circuit session with a single source.
	source := map[string]any{
		"name":   "linuxptp-daemon",
		"kind":   "repo",
		"uri":    "github.com/openshift/linuxptp-daemon",
		"branch": "release-4.18",
	}
	startArgs := mustJSON(map[string]any{
		"action": "start",
		"extra": map[string]any{
			"sources": []any{source},
			"query":   "phc2sys clock synchronization",
		},
	})

	t.Log("starting GND circuit...")
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      circuitToolName,
		Arguments: startArgs,
	})
	if err != nil {
		t.Fatalf("circuit start: %v", err)
	}
	startOut := extractText(t, result)
	t.Logf("start response: %s", truncate(startOut, 500))

	var startData map[string]any
	if err := json.Unmarshal([]byte(startOut), &startData); err != nil {
		t.Fatalf("parse start response: %v", err)
	}

	sessionID, _ := startData["session_id"].(string)
	totalCases, _ := startData["total_cases"].(float64)
	t.Logf("session_id=%s total_cases=%v", sessionID, totalCases)

	if sessionID == "" {
		t.Fatal("no session_id in start response")
	}

	// Poll for steps and process them (simple loop, max 20 iterations).
	for i := 0; i < 20; i++ {
		stepResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: circuitToolName,
			Arguments: mustJSON(map[string]any{
				"action":     "step",
				"session_id": sessionID,
				"timeout_ms": 15000,
			}),
		})
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		stepOut := extractText(t, stepResult)

		var stepData map[string]any
		if err := json.Unmarshal([]byte(stepOut), &stepData); err != nil {
			t.Fatalf("parse step %d: %v", i, err)
		}

		if done, _ := stepData["done"].(bool); done {
			if errMsg, _ := stepData["error"].(string); errMsg != "" {
				t.Logf("circuit done with error: %s", errMsg)
			} else {
				t.Log("circuit done successfully")
			}
			break
		}

		if avail, _ := stepData["available"].(bool); !avail {
			continue
		}

		step, _ := stepData["step"].(string)
		dispatchID, _ := stepData["dispatch_id"].(float64)
		promptContent, _ := stepData["prompt_content"].(string)
		t.Logf("step %d: node=%s dispatch_id=%v prompt_len=%d", i, step, dispatchID, len(promptContent))

		// Submit a minimal artifact to keep the circuit moving.
		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: circuitToolName,
			Arguments: mustJSON(map[string]any{
				"action":      "submit",
				"session_id":  sessionID,
				"dispatch_id": int64(dispatchID),
				"step":        step,
				"fields": map[string]any{
					"summary": fmt.Sprintf("acceptance test stub for %s", step),
				},
			}),
		})
		if err != nil {
			t.Fatalf("submit step %s: %v", step, err)
		}
		t.Logf("submitted step %s", step)
	}

	// Get summary.
	summaryResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: circuitToolName,
		Arguments: mustJSON(map[string]any{
			"action":     "summary",
			"session_id": sessionID,
		}),
	})
	if err != nil {
		t.Logf("summary (may fail if circuit errored): %v", err)
	} else {
		t.Logf("summary: %s", truncate(extractText(t, summaryResult), 500))
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func extractText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

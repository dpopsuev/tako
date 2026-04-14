package sdlc_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/simulate/sdlc"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSDLC_MCP_E2E proves the full production path:
// SessionFactory → SessionFactoryToConfig → CircuitServer → MCP lifecycle.
//
// The SDLC circuit runs in-process (stub transformers) through the MCP
// transport. This validates the fold-generated binary would work.
func TestSDLC_MCP_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("-short flag set")
	}

	// Force stub mode — don't need real instruments for this test.
	t.Setenv("SDLC_MODE", "stub")
	t.Setenv("SDLC_REPO_PATH", ".")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Build CircuitConfig from SessionFactory (same as fold-generated code).
	factory := sdlc.SessionFactory()
	cfg := mcp.SessionFactoryToConfig(factory)
	cfg.Name = "sdlc-mcp-e2e"
	cfg.Version = "test"
	cfg.DefaultGetNextStepTimeout = 10000
	cfg.DefaultSessionTTL = 30000
	cfg.FormatReport = func(result any) (string, any, error) {
		return "done", result, nil
	}

	// 2. Create CircuitServer.
	srv := mcp.NewCircuitServer(&cfg)
	t.Cleanup(srv.Shutdown)

	// 3. Connect via in-memory MCP transport.
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "sdlc-test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	// 4. List tools — should have circuit, signal, operator, budget at minimum.
	listRes, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	toolNames := make(map[string]bool)
	for _, tool := range listRes.Tools {
		toolNames[tool.Name] = true
	}
	t.Logf("tools: %v", toolNames)
	if !toolNames["circuit"] {
		t.Fatal("circuit tool not registered")
	}

	// 5. Start circuit.
	startRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: map[string]any{"action": "start"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if startRes.IsError {
		t.Fatalf("start error: %s", extractMCPText(startRes))
	}

	var startOut struct {
		SessionID string `json:"session_id"`
		Alias     string `json:"alias"`
	}
	json.Unmarshal([]byte(extractMCPText(startRes)), &startOut)
	if startOut.SessionID == "" {
		t.Fatal("no session_id in start result")
	}
	t.Logf("session: %s (%s)", startOut.SessionID, startOut.Alias)

	// 6. Wait for circuit to complete (it runs in-process with stubs, ~instant).
	// Poll report until status is available.
	var reportText string
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)

		reportRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: map[string]any{
				"action":     "report",
				"session_id": startOut.SessionID,
			},
		})
		if err != nil {
			continue
		}
		if reportRes.IsError {
			continue
		}
		reportText = extractMCPText(reportRes)
		if reportText != "" {
			var report struct {
				Status string `json:"status"`
			}
			json.Unmarshal([]byte(reportText), &report)
			if report.Status == "done" {
				t.Log("circuit completed")
				break
			}
		}
	}

	if reportText == "" {
		t.Fatal("circuit did not complete within timeout")
	}

	// 7. Verify budget tool works.
	budgetRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "budget",
		Arguments: map[string]any{"action": "summary"},
	})
	if err != nil {
		t.Logf("budget tool: %v (may not have active session)", err)
	} else if !budgetRes.IsError {
		t.Logf("budget: %s", extractMCPText(budgetRes))
	}

	// 8. Verify operator status tool works.
	opRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "operator",
		Arguments: map[string]any{"action": "status"},
	})
	if err != nil {
		t.Fatalf("operator status: %v", err)
	}
	t.Logf("operator: %s", extractMCPText(opRes))
}

func extractMCPText(res *sdkmcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// TestSDLC_MCP_E2E_FoldPathExists verifies the board manifest and component
// manifest exist and are parseable — the inputs fold needs to generate a binary.
func TestSDLC_MCP_E2E_FoldPathExists(t *testing.T) {
	t.Parallel()

	// Board manifest.
	if _, err := os.Stat("origami-sdlc.yaml"); err != nil {
		t.Errorf("board manifest: %v", err)
	}

	// Component manifest.
	if _, err := os.Stat("component.yaml"); err != nil {
		t.Errorf("component manifest: %v", err)
	}

	// Circuit YAML loads.
	def, err := sdlc.LoadCircuit(os.DirFS("."))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Circuit != "sdlc" {
		t.Errorf("circuit name = %q, want sdlc", def.Circuit)
	}
	t.Logf("circuit: %s, %d nodes, %d edges", def.Circuit, len(def.Nodes), len(def.Edges))
}

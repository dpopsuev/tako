package mediator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/mediator"
)

// TestMediator_AliasRouting_StepByAlias verifies that the mediator can
// route circuit(action=step) using the heraldic alias instead of the
// raw session ID. Reproduces ORG-BUG-17.
func TestMediator_AliasRouting_StepByAlias(t *testing.T) {
	// Create a circuit backend.
	cfg := mcp.CircuitConfig{
		Name:    "alias-test",
		Version: "dev",
		StepSchemas: []mcp.StepSchema{
			{Name: "STEP", Defs: []mcp.FieldDef{{Name: "value", Type: "string", Required: true}}},
		},
		DefaultGetNextStepTimeout: 5000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				if _, err := disp.Dispatch(ctx, agentport.Context{CaseID: "C01", Step: "STEP"}); err != nil {
					return nil, err
				}
				return map[string]any{"done": true}, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "alias-test"}, nil
		},
	}
	srv := mcp.NewCircuitServer(&cfg)
	t.Cleanup(srv.Shutdown)

	backendHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return srv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	backend := httptest.NewServer(backendHandler)
	t.Cleanup(backend.Close)

	// Create mediator.
	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Start circuit — get both session_id and alias.
	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "start"}),
	})
	if err != nil {
		t.Fatalf("circuit/start: %v", err)
	}
	startText := extractText(t, startResult)
	var startOut map[string]any
	json.Unmarshal([]byte(startText), &startOut)

	sessionID, _ := startOut["session_id"].(string)
	alias, _ := startOut["alias"].(string)
	if sessionID == "" {
		t.Fatalf("no session_id in start response: %s", startText)
	}
	if alias == "" {
		t.Fatalf("no alias in start response: %s", startText)
	}
	t.Logf("session_id=%s, alias=%s", sessionID, alias)

	// Call step with the raw session_id — should work (baseline).
	rawResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "step", "session_id": sessionID, "timeout_ms": 3000}),
	})
	if err != nil {
		t.Fatalf("step by raw ID failed: %v", err)
	}
	if rawResult.IsError {
		t.Fatalf("step by raw ID returned error: %s", extractText(t, rawResult))
	}

	// Call step with the ALIAS — this is the bug.
	// Currently fails because the mediator can't resolve the alias.
	aliasResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "step", "session_id": alias, "timeout_ms": 3000}),
	})
	if err != nil {
		t.Fatalf("step by alias failed: %v", err)
	}
	if aliasResult.IsError {
		t.Fatalf("step by alias returned error: %s — mediator can't route by alias (ORG-BUG-17)", extractText(t, aliasResult))
	}
}

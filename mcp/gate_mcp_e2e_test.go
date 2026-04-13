package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/testkit/stubs"
	"github.com/dpopsuev/troupe/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestGateMCP_E2E_ApproveViaToolLayer exercises the full gate stack through MCP:
//
//	circuit walks → parks at gated node → approval tool lists pending →
//	approval tool approves → store reflects approved state.
//
// This is TSK-694: the capstone test proving the approval gate works end-to-end
// through the MCP transport layer that agents actually use.
//
//nolint:gocyclo // E2E test — sequential multi-step flow is inherently complex
func TestGateMCP_E2E_ApproveViaToolLayer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store := stubs.NewMemoryApprovalStore()

	// Circuit: process → deploy (gated) → _done.
	def := &circuit.CircuitDef{
		Circuit: "gate-mcp-e2e",
		Start:   "process",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "process", Instrument: "transformer", Action: "passthrough"},
			{Name: "deploy", Instrument: "transformer", Action: "passthrough", Gate: engine.GateApproval},
		},
		Edges: []circuit.EdgeDef{
			{ID: "process-deploy", From: "process", To: "deploy"},
			{ID: "deploy-done", From: "deploy", To: "_done"},
		},
	}

	// walkDone signals when the walk completes or parks.
	walkDone := make(chan error, 1)

	cfg := &mcp.CircuitConfig{
		Name:                      "gate-mcp-e2e",
		Version:                   "test",
		StepSchemas:               []mcp.StepSchema{{Name: "GATE", Defs: []mcp.FieldDef{{Name: "result", Type: "string", Required: true}}}},
		ApprovalStore:             store,
		DefaultGetNextStepTimeout: 5000,
		DefaultSessionTTL:         30000,
		CreateSession: func(_ context.Context, _ mcp.StartParams, _ *dispatch.MuxDispatcher, _ signal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				reg := &engine.GraphRegistries{
					ApprovalStore: store,
				}
				g, err := engine.BuildGraph(def, reg)
				if err != nil {
					return nil, err
				}

				walker := circuit.NewProcessWalker("gate-mcp-e2e")
				walkErr := g.Walk(ctx, walker, "process")

				// Signal the walk result.
				walkDone <- walkErr

				if errors.Is(walkErr, engine.ErrWalkInterrupted) {
					return map[string]any{"status": "parked", "node": walker.State().CurrentNode}, nil
				}
				if walkErr != nil {
					return nil, walkErr
				}
				return map[string]any{"status": "completed"}, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "gate-e2e"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "done", result, nil
		},
	}

	srv := newTestServer(t, cfg)
	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	// 1. Start circuit — the walk will park at the gated node.
	startResult := callTool(ctx, t, session, "start_circuit", map[string]any{})
	sessionID, _ := startResult["session_id"].(string)
	if sessionID == "" {
		t.Fatal("no session_id in start result")
	}
	t.Logf("session: %s", sessionID)

	// Wait for walk to park.
	select {
	case walkErr := <-walkDone:
		if !errors.Is(walkErr, engine.ErrWalkInterrupted) {
			t.Fatalf("walk: expected ErrWalkInterrupted, got %v", walkErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("walk did not park within 5s")
	}

	// 2. List pending approvals via MCP tool.
	listResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "approval",
		Arguments: map[string]any{"action": "list"},
	})
	if err != nil {
		t.Fatalf("approval list: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("approval list returned error")
	}

	var listOut struct {
		Items []struct {
			ID       string `json:"id"`
			NodeName string `json:"node_name"`
			Status   string `json:"status"`
		} `json:"items"`
		Count int `json:"count"`
	}
	tc, _ := listResult.Content[0].(*sdkmcp.TextContent)
	json.Unmarshal([]byte(tc.Text), &listOut)

	if listOut.Count != 1 {
		t.Fatalf("pending count = %d, want 1", listOut.Count)
	}
	if listOut.Items[0].NodeName != "deploy" {
		t.Errorf("pending node = %q, want deploy", listOut.Items[0].NodeName)
	}
	if listOut.Items[0].Status != "pending" {
		t.Errorf("status = %q, want pending", listOut.Items[0].Status)
	}
	approvalID := listOut.Items[0].ID
	t.Logf("approval ID: %s", approvalID)

	// 3. Approve via MCP tool.
	approveResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "approval",
		Arguments: map[string]any{
			"action":   "approve",
			"id":       approvalID,
			"comment":  "LGTM — deploy approved",
			"operator": "test-operator",
		},
	})
	if err != nil {
		t.Fatalf("approval approve: %v", err)
	}
	if approveResult.IsError {
		t.Fatal("approval approve returned error")
	}

	var approveOut struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	tc2, _ := approveResult.Content[0].(*sdkmcp.TextContent)
	json.Unmarshal([]byte(tc2.Text), &approveOut)

	if approveOut.Status != "approved" {
		t.Errorf("approve result status = %q, want approved", approveOut.Status)
	}

	// 4. Verify via get — item is approved with comment.
	getResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "approval",
		Arguments: map[string]any{"action": "get", "id": approvalID},
	})
	if err != nil {
		t.Fatalf("approval get: %v", err)
	}

	var getOut struct {
		Status   string `json:"status"`
		NodeName string `json:"node_name"`
		Decision struct {
			Status   string `json:"status"`
			Comment  string `json:"comment"`
			Operator string `json:"operator"`
		} `json:"decision"`
	}
	tc3, _ := getResult.Content[0].(*sdkmcp.TextContent)
	json.Unmarshal([]byte(tc3.Text), &getOut)

	if getOut.Status != "approved" {
		t.Errorf("get status = %q", getOut.Status)
	}
	if getOut.Decision.Comment != "LGTM — deploy approved" {
		t.Errorf("decision comment = %q", getOut.Decision.Comment)
	}
	if getOut.Decision.Operator != "test-operator" {
		t.Errorf("decision operator = %q", getOut.Decision.Operator)
	}

	// 5. List pending — should be empty now.
	listResult2, _ := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "approval",
		Arguments: map[string]any{"action": "list"},
	})
	tc4, _ := listResult2.Content[0].(*sdkmcp.TextContent)
	var listOut2 struct {
		Count int `json:"count"`
	}
	json.Unmarshal([]byte(tc4.Text), &listOut2)
	if listOut2.Count != 0 {
		t.Errorf("pending after approve = %d, want 0", listOut2.Count)
	}
}

package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/dpopsuev/tako/dispatch"
	"github.com/dpopsuev/tako/mcp"
	"github.com/dpopsuev/tako/resource"
	"github.com/dpopsuev/tangle/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newResourceServer() *mcp.CircuitServer {
	domainFS := fstest.MapFS{
		"circuits/alpha.yaml": &fstest.MapFile{
			Data: []byte("kind: Schematic\nversion: v1\nmetadata:\n  name: alpha\ncircuit: alpha\nnodes:\n  - name: recall\n    handler: transformer:recall\nedges: []\nstart: recall\ndone: recall\n"),
		},
		"scorecards/alpha.yaml": &fstest.MapFile{
			Data: []byte("kind: Scorecard\nversion: v1\nmetadata:\n  name: alpha\nmetrics:\n  - id: M1\n    name: accuracy\n    scorer: exact_match\n    threshold: 0.7\n"),
		},
		"scenarios/ptp.yaml": &fstest.MapFile{
			Data: []byte("kind: Scenario\nversion: v1\nmetadata:\n  name: ptp\ncases: []\n"),
		},
	}

	return mcp.NewCircuitServer(&mcp.CircuitConfig{
		Name:        "resource-test",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		CreateSession: func(_ context.Context, _ mcp.StartParams, _ *dispatch.MuxDispatcher, _ signal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(_ context.Context) (any, error) { return nil, nil }, mcp.SessionMeta{}, nil
		},
		DomainFS:         domainFS,
		ResourceRegistry: resource.DefaultRegistry(),
	})
}

func callResourceToolRaw(ctx context.Context, t *testing.T, session *sdkmcp.ClientSession, args map[string]any) []byte {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "resource",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(resource): %v", err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("resource tool error: %s", tc.Text)
			}
		}
		t.Fatalf("resource tool returned error")
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return []byte(tc.Text)
		}
	}
	t.Fatal("no text content in resource result")
	return nil
}

func TestResourceTool_Kinds(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{"action": "kinds"})
	var kinds []map[string]any
	if err := json.Unmarshal(raw, &kinds); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(kinds) < 10 {
		t.Errorf("expected 10+ kinds, got %d", len(kinds))
	}
	// Check that schematic is present with merge=false
	found := false
	for _, k := range kinds {
		if k["kind"] == "Schematic" {
			found = true
		}
	}
	if !found {
		t.Error("schematic not in kinds list")
	}
}

func TestResourceTool_List(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{"action": "list"})
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 resources, got %d", len(list))
	}
}

func TestResourceTool_ListFilterByKind(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{"action": "list", "kind": "Scorecard"})
	var list []map[string]any
	json.Unmarshal(raw, &list)
	if len(list) != 1 {
		t.Errorf("expected 1 scorecard, got %d", len(list))
	}
}

func TestResourceTool_Get(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{
		"action": "get",
		"kind":   "Scenario",
		"name":   "ptp",
	})
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	res, ok := result["resource"].(map[string]any)
	if !ok {
		t.Fatal("expected resource in result")
	}
	if res["kind"] != "Scenario" {
		t.Errorf("kind = %v", res["kind"])
	}
}

func TestResourceTool_Validate(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{
		"action": "validate",
		"kind":   "Scenario",
		"name":   "ptp",
	})
	var result map[string]any
	json.Unmarshal(raw, &result)
	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
}

func TestResourceTool_Diff(t *testing.T) {
	ctx := context.Background()
	srv := newResourceServer()
	session := connectInMemory(ctx, t, srv)

	raw := callResourceToolRaw(ctx, t, session, map[string]any{
		"action": "diff",
		"file_a": "circuits/alpha.yaml",
		"file_b": "scorecards/alpha.yaml",
	})
	var result map[string]any
	json.Unmarshal(raw, &result)
	changes, _ := result["changes"].(float64)
	if changes == 0 {
		t.Error("expected non-zero changes between different files")
	}
}

func TestResourceTool_NotRegisteredWithoutConfig(t *testing.T) {
	ctx := context.Background()
	srv := mcp.NewCircuitServer(&mcp.CircuitConfig{
		Name:        "no-resource-test",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		CreateSession: func(_ context.Context, _ mcp.StartParams, _ *dispatch.MuxDispatcher, _ signal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(_ context.Context) (any, error) { return nil, nil }, mcp.SessionMeta{}, nil
		},
		// ResourceRegistry intentionally nil
	})
	session := connectInMemory(ctx, t, srv)

	_, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "resource",
		Arguments: map[string]any{"action": "kinds"},
	})
	if err == nil {
		t.Error("expected error calling resource tool when not configured")
	}
}

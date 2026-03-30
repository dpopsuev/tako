package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/prompt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newPromptServer(store prompt.Store) *mcp.CircuitServer {
	return mcp.NewCircuitServer(&mcp.CircuitConfig{
		Name:        "prompt-test",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		CreateSession: func(_ context.Context, _ mcp.StartParams, _ *dispatch.MuxDispatcher, _ agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(_ context.Context) (any, error) { return nil, nil }, mcp.SessionMeta{}, nil
		},
		PromptStore: store,
	})
}

// callPromptToolRaw calls the prompt tool and returns the raw JSON text.
func callPromptToolRaw(ctx context.Context, t *testing.T, session *sdkmcp.ClientSession, args map[string]any) []byte {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "prompt",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(prompt): %v", err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("prompt tool error: %s", tc.Text)
			}
		}
		t.Fatalf("prompt tool returned error")
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return []byte(tc.Text)
		}
	}
	t.Fatal("no text content in prompt result")
	return nil
}

func TestPromptTool_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	store := prompt.NewLiveStore()
	srv := newPromptServer(store)
	session := connectInMemory(ctx, t, srv)

	// Create a prompt.
	callPromptToolRaw(ctx, t, session, map[string]any{
		"action":  "create",
		"name":    "triage",
		"step":    "f1",
		"content": "# Triage\n\n## Task\n\nClassify.",
	})

	// Get it back.
	raw := callPromptToolRaw(ctx, t, session, map[string]any{
		"action": "get",
		"name":   "triage",
	})
	var p prompt.Prompt
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Name != "triage" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}
	if len(p.Sections) != 2 {
		t.Errorf("Sections = %d, want 2", len(p.Sections))
	}
}

func TestPromptTool_List(t *testing.T) {
	ctx := context.Background()
	store := prompt.NewLiveStore()
	srv := newPromptServer(store)
	session := connectInMemory(ctx, t, srv)

	store.Create("a", "s1", "# A\n\ncontent")
	store.Create("b", "s2", "# B\n\ncontent")

	raw := callPromptToolRaw(ctx, t, session, map[string]any{"action": "list"})
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("list = %d, want 2", len(list))
	}
}

func TestPromptTool_UpdateAndRollback(t *testing.T) {
	ctx := context.Background()
	store := prompt.NewLiveStore()
	srv := newPromptServer(store)
	session := connectInMemory(ctx, t, srv)

	store.Create("triage", "f1", "version 1")

	// Update to v2.
	callPromptToolRaw(ctx, t, session, map[string]any{
		"action":  "update",
		"name":    "triage",
		"content": "version 2",
	})

	// Verify v2.
	raw := callPromptToolRaw(ctx, t, session, map[string]any{"action": "get", "name": "triage"})
	var p prompt.Prompt
	json.Unmarshal(raw, &p)
	if p.Version != 2 {
		t.Errorf("Version = %d, want 2", p.Version)
	}

	// Rollback to v1.
	raw = callPromptToolRaw(ctx, t, session, map[string]any{
		"action":  "rollback",
		"name":    "triage",
		"version": 1,
	})
	json.Unmarshal(raw, &p)
	if p.Version != 3 {
		t.Errorf("Rollback version = %d, want 3", p.Version)
	}
	if p.Content != "version 1" {
		t.Errorf("Content = %q, want %q", p.Content, "version 1")
	}
}

func TestPromptTool_NotRegisteredWithoutStore(t *testing.T) {
	// When PromptStore is nil, the prompt tool should not be registered.
	ctx := context.Background()
	srv := mcp.NewCircuitServer(&mcp.CircuitConfig{
		Name:        "no-prompt-test",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		CreateSession: func(_ context.Context, _ mcp.StartParams, _ *dispatch.MuxDispatcher, _ agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(_ context.Context) (any, error) { return nil, nil }, mcp.SessionMeta{}, nil
		},
		// PromptStore intentionally nil
	})
	session := connectInMemory(ctx, t, srv)

	_, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "prompt",
		Arguments: map[string]any{"action": "list"},
	})
	if err == nil {
		t.Error("expected error calling prompt tool when PromptStore is nil")
	}
}

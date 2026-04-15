package operator_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/mcpserver"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/origami/operator"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestScribeObserver_DetectsMatureTasks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stub Scribe server returns 2 mature tasks.
	srv := mcpserver.NewServer("stub-scribe", "v0.0.1").
		Tool(server.ToolMeta{Name: "artifact", Description: "Stub artifact"}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult(`[{"id":"TSK-1","title":"fix auth","status":"mature","priority":"high"},{"id":"TSK-2","title":"add tests","status":"mature","priority":"medium"}]`), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "scribe", clientTransport); err != nil {
		t.Fatalf("register: %v", err)
	}

	obs := operator.NewScribeObserver(registry)
	state, err := obs.Observe()
	if err != nil {
		t.Fatalf("observe: %v", err)
	}

	// ScanFindings = number of mature tasks = drift signal.
	if state.ScanFindings != 2 {
		t.Errorf("findings = %d, want 2 (mature tasks)", state.ScanFindings)
	}
}

func TestScribeObserver_NoPendingTasks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stub Scribe returns empty list.
	srv := mcpserver.NewServer("stub-scribe", "v0.0.1").
		Tool(server.ToolMeta{Name: "artifact", Description: "Stub artifact"}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult(`[]`), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	adapter.RegisterMCP(ctx, "scribe", clientTransport)

	obs := operator.NewScribeObserver(registry)
	state, err := obs.Observe()
	if err != nil {
		t.Fatalf("observe: %v", err)
	}

	if state.ScanFindings != 0 {
		t.Errorf("findings = %d, want 0 (no mature tasks)", state.ScanFindings)
	}
}

func TestScribeObserver_ImplementsObserver(t *testing.T) {
	var _ operator.Observer = (*operator.ScribeObserver)(nil)
}

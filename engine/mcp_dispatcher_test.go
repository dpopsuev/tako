package engine_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/tool"
	mcpserver "github.com/dpopsuev/tako/tool/mcp"
	"github.com/dpopsuev/tako/tool/server"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPDispatcher_E2E(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Build an instrument MCP server with mcpserver.
	srv := mcpserver.NewServer("test-instrument", "v1.0.0").
		Tool(server.ToolMeta{
			Name:        "analyze",
			Description: "Analyze code",
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.TextResult(`{"findings":3,"status":"ok"}`), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	// 2. Create MCPDispatcher.
	dispatcher, err := engine.NewMCPDispatcher(ctx, "test-instrument", "analyze", clientTransport)
	if err != nil {
		t.Fatalf("NewMCPDispatcher: %v", err)
	}

	// 3. Dispatch action.
	output, err := dispatcher.Dispatch(ctx, json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// 4. Verify output.
	var parsed struct {
		Findings int    `json:"findings"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, output)
	}
	if parsed.Findings != 3 || parsed.Status != "ok" {
		t.Errorf("output = %+v", parsed)
	}
}

func TestMCPDispatcher_ImplementsInstrumentDispatcher(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := mcpserver.NewServer("iface-test", "v1.0.0").
		Tool(server.ToolMeta{Name: "ping", Description: "Ping"}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult("pong"), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	dispatcher, err := engine.NewMCPDispatcher(ctx, "iface-test", "ping", clientTransport)
	if err != nil {
		t.Fatalf("NewMCPDispatcher: %v", err)
	}

	// Verify it satisfies InstrumentDispatcher.
	var _ engine.InstrumentDispatcher = dispatcher
}

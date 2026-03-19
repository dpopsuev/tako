package subprocess_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStreamableHTTP_RoundTrip(t *testing.T) {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-gnd", Version: "v0.1.0"},
		nil,
	)
	server.AddTool(
		&sdkmcp.Tool{
			Name:        "gnd_search",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var args struct{ Query string }
			json.Unmarshal(req.Params.Arguments, &args)
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "found: " + args.Query}},
			}, nil
		},
	)

	handler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx := t.Context()

	transport := &sdkmcp.StreamableClientTransport{
		Endpoint: httpServer.URL,
	}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "gnd_search",
		Arguments: map[string]any{"query": "ptp holdover"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty result")
	}
	text := result.Content[0].(*sdkmcp.TextContent).Text
	if text != "found: ptp holdover" {
		t.Errorf("got %q, want %q", text, "found: ptp holdover")
	}
}

func TestStreamableHTTP_MultipleTools(t *testing.T) {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-gnd", Version: "v0.1.0"},
		nil,
	)
	server.AddTool(
		&sdkmcp.Tool{
			Name:        "gnd_ensure",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
			}, nil
		},
	)
	server.AddTool(
		&sdkmcp.Tool{
			Name:        "gnd_list",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"root":{"type":"string"}}}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var args struct{ Root string }
			json.Unmarshal(req.Params.Arguments, &args)
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "listing: " + args.Root}},
			}, nil
		},
	)

	handler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: httpServer.URL}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	r1, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "gnd_ensure"})
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if r1.Content[0].(*sdkmcp.TextContent).Text != "ok" {
		t.Errorf("ensure got %q", r1.Content[0].(*sdkmcp.TextContent).Text)
	}

	r2, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "gnd_list",
		Arguments: map[string]any{"root": "/src"},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if r2.Content[0].(*sdkmcp.TextContent).Text != "listing: /src" {
		t.Errorf("list got %q", r2.Content[0].(*sdkmcp.TextContent).Text)
	}
}

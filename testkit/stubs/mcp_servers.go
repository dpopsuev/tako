package stubs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/tako/tool"
	mcpserver "github.com/dpopsuev/tako/tool/mcp"
	"github.com/dpopsuev/tako/tool/server"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// StubScribeServer creates a minimal Scribe MCP server for testing.
// Returns a client transport to connect via MCPAdapter.
func StubScribeServer(t *testing.T) sdkmcp.Transport {
	t.Helper()
	return stubMCPServer(t, "scribe", map[string]string{
		"artifact": `{"items":[],"count":0}`,
		"graph":    `{"nodes":[],"edges":[]}`,
		"admin":    `{"motd":"stub scribe ready"}`,
	})
}

// StubLexServer creates a minimal Lex MCP server for testing.
func StubLexServer(t *testing.T) sdkmcp.Transport {
	t.Helper()
	return stubMCPServer(t, "lex", map[string]string{
		"lexicon": `{"rules":[],"skills":[]}`,
		"config":  `{"sources":[]}`,
	})
}

// StubLocusServer creates a minimal Locus MCP server for testing.
func StubLocusServer(t *testing.T) sdkmcp.Transport {
	t.Helper()
	return stubMCPServer(t, "locus", map[string]string{
		"codograph": `{"cache_key":"stub-key"}`,
		"analysis":  `{"components":[]}`,
		"clinic":    `{"patterns":[]}`,
		"lint":      `{"issues":[]}`,
		"triage":    `{"tools":[]}`,
	})
}

func stubMCPServer(t *testing.T, name string, tools map[string]string) sdkmcp.Transport {
	t.Helper()

	srv := mcpserver.NewServer("stub-"+name, "v0.0.1")
	for toolName, response := range tools {
		resp := response
		srv.Tool(server.ToolMeta{
			Name:        toolName,
			Description: "Stub " + toolName,
		}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult(resp), nil
		})
	}

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(context.Background(), serverTransport) }()

	return clientTransport
}

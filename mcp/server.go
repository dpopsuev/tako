package mcp

import (
	"github.com/dpopsuev/battery/mcpserver"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the Battery MCP server framework. Domains create a server
// with NewServer, register tools via the builder API or raw SDK access,
// then run with Serve().
type Server struct {
	Battery   *mcpserver.Server
	MCPServer *sdkmcp.Server // raw SDK access for typed handlers
}

// NewServer creates an MCP server backed by Battery's mcpserver framework.
// Auto-Observable wrapping, panic recovery, and result helpers are built in.
func NewServer(name, version string) *Server {
	batt := mcpserver.NewServer(name, version)
	return &Server{
		Battery:   batt,
		MCPServer: batt.SDK(),
	}
}

package engine

// Category: Execution — MCP instrument dispatcher.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dpopsuev/tako/tool"
	battmcp "github.com/dpopsuev/tako/tool/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrMCPDispatch is returned when an MCP instrument dispatch fails.
var ErrMCPDispatch = errors.New("mcp dispatch")

// MCPDispatcher implements InstrumentDispatcher for dispatch: mcp instruments.
// It connects to an MCP server via transport and dispatches actions as tool calls.
type MCPDispatcher struct {
	adapter    *battmcp.MCPAdapter
	registry   *tool.Registry
	serverName string
	actionName string
}

// NewMCPDispatcher creates a dispatcher that connects to an MCP server and
// dispatches the named action as a tool call.
func NewMCPDispatcher(ctx context.Context, serverName, actionName string, transport sdkmcp.Transport) (*MCPDispatcher, error) {
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)

	if err := adapter.RegisterMCP(ctx, serverName, transport); err != nil {
		return nil, err
	}

	return &MCPDispatcher{
		adapter:    adapter,
		registry:   registry,
		serverName: serverName,
		actionName: actionName,
	}, nil
}

// Dispatch executes the action as an MCP tool call.
func (d *MCPDispatcher) Dispatch(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	toolName := d.serverName + "." + d.actionName

	result, err := d.registry.Execute(ctx, toolName, input)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(result.Text()), nil
}

// LazyMCPDispatcher defers MCP connection to first Dispatch call.
// Used by createDispatcher at build time when no context is available.
type LazyMCPDispatcher struct {
	serverName string
	actionName string
	endpoint   string
	inner      *MCPDispatcher
}

// Dispatch connects on first call, then delegates to MCPDispatcher.
func (d *LazyMCPDispatcher) Dispatch(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.inner == nil {
		parts := strings.Fields(d.endpoint)
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec // endpoint from trusted manifest
		transport := &sdkmcp.CommandTransport{Command: cmd}
		inner, err := NewMCPDispatcher(ctx, d.serverName, d.actionName, transport)
		if err != nil {
			return nil, fmt.Errorf("%w: connect to %s: %w", ErrMCPDispatch, d.endpoint, err)
		}
		d.inner = inner
	}
	return d.inner.Dispatch(ctx, input)
}

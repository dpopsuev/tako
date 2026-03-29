// Package subprocess manages MCP server processes as child subprocesses.
// It wraps the MCP SDK's CommandTransport with lifecycle management:
// start, stop, restart, health checking, and typed tool calls.
package subprocess

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server manages a schematic running as a child process, communicating via
// MCP over stdio. It handles lifecycle (start/stop/restart) and provides
// typed tool call access.
type Server struct {
	BinaryPath        string
	Args              []string
	Env               []string // additional env vars (appended to os.Environ)
	Connector         *MCPConnector
	TerminateDuration time.Duration // SIGTERM→SIGKILL escalation delay (default 5s)
	PingTimeout       time.Duration // health check ping deadline (default 2s)

	mu      sync.Mutex
	session *sdkmcp.ClientSession
	cmd     *exec.Cmd
}

// Start launches the subprocess and connects an MCP client to it.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return ErrSubprocessAlreadyStarted
	}

	cmd := exec.CommandContext(ctx, s.BinaryPath, s.Args...) //nolint:gosec // binary path is from trusted config
	if len(s.Env) > 0 {
		cmd.Env = append(cmd.Environ(), s.Env...)
	}

	terminateDur := s.TerminateDuration
	if terminateDur == 0 {
		terminateDur = 5 * time.Second
	}

	transport := &sdkmcp.CommandTransport{
		Command:           cmd,
		TerminateDuration: terminateDur,
	}

	conn := s.Connector
	if conn == nil {
		conn = DefaultConnector()
	}

	session, err := conn.Connect(ctx, transport)
	if err != nil {
		return fmt.Errorf("connecting to subprocess: %w", err)
	}

	s.cmd = cmd
	s.session = session
	return nil
}

// Stop gracefully shuts down the subprocess. The MCP SDK's CommandTransport
// handles SIGTERM/SIGKILL escalation.
func (s *Server) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil
	}

	err := s.session.Close()
	s.session = nil
	s.cmd = nil
	return err
}

// Restart stops the current subprocess and starts a new one. If stop fails,
// the restart is aborted.
func (s *Server) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("stopping for restart: %w", err)
	}
	return s.Start(ctx)
}

// Healthy returns true if the subprocess is running and the MCP session
// responds to a ping.
func (s *Server) Healthy(ctx context.Context) bool {
	s.mu.Lock()
	session := s.session
	s.mu.Unlock()

	if session == nil {
		return false
	}

	pingTimeout := s.PingTimeout
	if pingTimeout == 0 {
		pingTimeout = 2 * time.Second
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	return session.Ping(pingCtx, nil) == nil
}

// CallTool invokes a tool on the subprocess MCP server and returns the result.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	s.mu.Lock()
	session := s.session
	s.mu.Unlock()

	if session == nil {
		return nil, ErrSubprocessNotStarted
	}

	return session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Session returns the underlying MCP client session, or nil if not connected.
func (s *Server) Session() *sdkmcp.ClientSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

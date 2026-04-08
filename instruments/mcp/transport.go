// Package mcp provides an MCP Streamable HTTP transport for Origami circuits.
// It implements toolkit.Transport by serving a CircuitServer over HTTP.
package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dpopsuev/origami/toolkit"
)

// MCPTransportConfig configures the MCP transport.
type MCPTransportConfig struct {
	Port int // Listen port (0 = ephemeral)
}

// MCPTransport implements toolkit.Transport by serving an HTTP server
// with /healthz and (when wired) /mcp endpoints.
type MCPTransport struct {
	config MCPTransportConfig

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

// NewMCPTransport creates an MCP transport with the given config.
func NewMCPTransport(cfg MCPTransportConfig) *MCPTransport {
	return &MCPTransport{config: cfg}
}

// Serve starts the HTTP server and blocks until ctx is canceled.
func (t *MCPTransport) Serve(ctx context.Context, handler toolkit.TransportHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})

	// TODO: when handler is non-nil, wire /mcp endpoint via
	// sdkmcp.NewStreamableHTTPHandler bridging TransportHandler to CircuitServer.

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", t.config.Port))
	if err != nil {
		return fmt.Errorf("mcp transport: listen: %w", err)
	}

	t.mu.Lock()
	t.listener = ln
	t.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	t.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- t.server.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		_ = t.server.Close()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the HTTP server.
func (t *MCPTransport) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	srv := t.server
	t.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// Port returns the actual listen port (useful when config.Port=0).
// Returns 0 if Serve hasn't been called yet.
func (t *MCPTransport) Port() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.listener == nil {
		return 0
	}
	return t.listener.Addr().(*net.TCPAddr).Port
}

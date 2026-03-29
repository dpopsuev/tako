package subprocess

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RemoteBackend implements SchematicBackend for an already-running MCP
// endpoint. Unlike Server and ContainerBackend, it does not own the
// remote process lifecycle — Start establishes a session, Stop closes
// it without killing the remote.
type RemoteBackend struct {
	Endpoint    string        // full URL, e.g. "http://127.0.0.1:9200/mcp"
	Connector   *MCPConnector // nil uses DefaultConnector()
	HTTPTimeout time.Duration // HTTP client timeout; defaults to 30s
	PingTimeout time.Duration // health check timeout; defaults to 2s

	mu      sync.Mutex
	session *sdkmcp.ClientSession
}

// Start connects to the remote MCP endpoint and establishes a session.
func (rb *RemoteBackend) Start(ctx context.Context) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.session != nil {
		return fmt.Errorf("%w: %q already connected", ErrRemote, rb.Endpoint)
	}

	httpTimeout := rb.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second
	}

	transport := &sdkmcp.StreamableClientTransport{
		Endpoint: rb.Endpoint,
		HTTPClient: &http.Client{
			Timeout: httpTimeout,
		},
	}

	c := rb.Connector
	if c == nil {
		c = DefaultConnector()
	}

	session, err := c.Connect(ctx, transport)
	if err != nil {
		return fmt.Errorf("remote %q: %w", rb.Endpoint, err)
	}

	rb.session = session
	return nil
}

// Stop closes the MCP session without affecting the remote process.
func (rb *RemoteBackend) Stop(_ context.Context) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.session == nil {
		return nil
	}

	err := rb.session.Close()
	rb.session = nil
	return err
}

// CallTool invokes a tool on the remote MCP server.
func (rb *RemoteBackend) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	rb.mu.Lock()
	session := rb.session
	rb.mu.Unlock()

	if session == nil {
		return nil, fmt.Errorf("%w: %q not connected", ErrRemote, rb.Endpoint)
	}
	return session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Healthy returns true if the MCP session responds to a ping.
func (rb *RemoteBackend) Healthy(ctx context.Context) bool {
	rb.mu.Lock()
	session := rb.session
	rb.mu.Unlock()

	if session == nil {
		return false
	}

	pingTimeout := rb.PingTimeout
	if pingTimeout == 0 {
		pingTimeout = 2 * time.Second
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return session.Ping(pingCtx, nil) == nil
}

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPActor runs the circuit via an in-process MCP server. The circuit
// executes through the full MCP dispatch path (SessionFactory →
// CircuitServer → MCP transport) instead of calling BatchWalk directly.
// This is the production execution path.
type MCPActor struct {
	factory engine.SessionFactory
	timeout time.Duration
}

// MCPActorOption configures the MCP actor.
type MCPActorOption func(*MCPActor)

// WithMCPTimeout sets the maximum duration for the circuit run.
func WithMCPTimeout(d time.Duration) MCPActorOption {
	return func(a *MCPActor) { a.timeout = d }
}

// NewMCPActor creates an actor that runs the circuit via MCP transport.
func NewMCPActor(factory engine.SessionFactory, opts ...MCPActorOption) *MCPActor {
	a := &MCPActor{
		factory: factory,
		timeout: 10 * time.Minute, //nolint:mnd // reasonable default for circuit execution
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Act implements Actor by creating an in-process MCP server, running the
// circuit through it, and polling for completion.
func (a *MCPActor) Act(_ DriftResult) (*RunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()

	start := time.Now()

	// Build CircuitServer from factory.
	cfg := mcp.SessionFactoryToConfig(a.factory)
	cfg.Name = "operator-mcp"
	cfg.Version = "v1"
	cfg.DefaultGetNextStepTimeout = 10000                 //nolint:mnd // 10s
	cfg.DefaultSessionTTL = int(a.timeout.Milliseconds()) //nolint:mnd // convert to ms
	cfg.FormatReport = func(result any) (string, any, error) {
		return "done", result, nil
	}

	srv := mcp.NewCircuitServer(&cfg)
	defer srv.Shutdown()

	// Connect via in-memory transport.
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		return &RunResult{Success: false, Duration: time.Since(start), Error: fmt.Sprintf("server connect: %v", err)}, nil
	}
	defer serverSession.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "operator-client", Version: "v1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		return &RunResult{Success: false, Duration: time.Since(start), Error: fmt.Sprintf("client connect: %v", err)}, nil
	}
	defer session.Close()

	// Start circuit.
	startRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: map[string]any{"action": "start"},
	})
	if err != nil {
		return &RunResult{Success: false, Duration: time.Since(start), Error: fmt.Sprintf("start: %v", err)}, nil
	}
	if startRes.IsError {
		return &RunResult{Success: false, Duration: time.Since(start), Error: "start returned error"}, nil
	}

	var startOut struct {
		SessionID string `json:"session_id"`
	}
	for _, c := range startRes.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			_ = json.Unmarshal([]byte(tc.Text), &startOut)
		}
	}

	// Poll report until done.
	for {
		select {
		case <-ctx.Done():
			return &RunResult{Success: false, Duration: time.Since(start), Error: "timeout"}, nil
		default:
		}

		time.Sleep(500 * time.Millisecond) //nolint:mnd // poll interval

		reportRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: map[string]any{
				"action":     "report",
				"session_id": startOut.SessionID,
			},
		})
		if err != nil || reportRes.IsError {
			continue
		}

		for _, c := range reportRes.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				var report struct {
					Status string `json:"status"`
				}
				_ = json.Unmarshal([]byte(tc.Text), &report)
				if report.Status == "done" {
					return &RunResult{Success: true, Duration: time.Since(start)}, nil
				}
			}
		}
	}
}

var _ Actor = (*MCPActor)(nil)

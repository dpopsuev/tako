// Package mediator implements the Origami Mediator — a session-aware MCP
// router that coordinates schematics via the Papercup protocol. Papercup
// tools are routed by circuit_type and session affinity. Non-Papercup tools
// are routed by tool name.
package mediator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

// PapercupTools enumerates the Papercup protocol tool names.
var PapercupTools = map[string]bool{
	"start_circuit":    true,
	"get_next_step":    true,
	"submit_step":      true,
	"get_report":       true,
	"emit_signal":      true,
	"get_signals":      true,
	"get_worker_health": true,
}

type routedTool struct {
	backendName string
	tool        sdkmcp.Tool
}

// BackendConfig describes a named backend MCP service.
type BackendConfig struct {
	Name        string
	Endpoint    string
	CircuitType string // if set, route start_circuit(circuit_type=X) to this backend
}

// Mediator proxies MCP tool calls to backend services.
// Papercup tools are routed by circuit_type (start_circuit) and
// session affinity (all other Papercup calls). Non-Papercup tools
// are routed by tool name.
type Mediator struct {
	mu         sync.RWMutex
	backends   map[string]*subprocess.RemoteBackend
	sessions   map[string]*sdkmcp.ClientSession
	toolRoutes map[string]routedTool // non-Papercup tool name → backend

	// Papercup session-affinity routing.
	circuitBackends map[string]string // circuit_type → backend name
	defaultBackend  string            // backend for start_circuit without circuit_type
	sessionAffinity map[string]string // session_id → backend name

	// Papercup tool schemas (registered once from first backend that has them).
	papercupSchemas map[string]sdkmcp.Tool
}

// New creates a Mediator that will connect to the given backends.
func New(configs []BackendConfig) *Mediator {
	gw := &Mediator{
		backends:        make(map[string]*subprocess.RemoteBackend, len(configs)),
		sessions:        make(map[string]*sdkmcp.ClientSession, len(configs)),
		toolRoutes:      make(map[string]routedTool),
		circuitBackends: make(map[string]string),
		sessionAffinity: make(map[string]string),
		papercupSchemas: make(map[string]sdkmcp.Tool),
	}
	for _, cfg := range configs {
		gw.backends[cfg.Name] = &subprocess.RemoteBackend{Endpoint: cfg.Endpoint}
		if cfg.CircuitType != "" {
			gw.circuitBackends[cfg.CircuitType] = cfg.Name
		} else {
			gw.defaultBackend = cfg.Name
		}
	}
	return gw
}

// Start connects to all backends and builds the routing tables.
func (gw *Mediator) Start(ctx context.Context) error {
	for name, rb := range gw.backends {
		if err := rb.Start(ctx); err != nil {
			return fmt.Errorf("backend %q: %w", name, err)
		}
	}

	for name, rb := range gw.backends {
		transport := &sdkmcp.StreamableClientTransport{Endpoint: rb.Endpoint}
		client := sdkmcp.NewClient(
			&sdkmcp.Implementation{Name: "origami-mediator", Version: "v0.1.0"},
			nil,
		)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			return fmt.Errorf("discover tools for %q: %w", name, err)
		}
		gw.sessions[name] = session

		tools, err := session.ListTools(ctx, nil)
		if err != nil {
			session.Close()
			return fmt.Errorf("list tools for %q: %w", name, err)
		}

		gw.mu.Lock()
		for _, tool := range tools.Tools {
			if PapercupTools[tool.Name] {
				// Store schema once (all backends have identical Papercup schemas).
				if _, exists := gw.papercupSchemas[tool.Name]; !exists {
					gw.papercupSchemas[tool.Name] = *tool
				}
			} else {
				gw.toolRoutes[tool.Name] = routedTool{backendName: name, tool: *tool}
			}
		}
		gw.mu.Unlock()
	}

	return nil
}

// Stop closes all discovery sessions and backend connections.
func (gw *Mediator) Stop(ctx context.Context) {
	for _, s := range gw.sessions {
		s.Close()
	}
	for _, rb := range gw.backends {
		rb.Stop(ctx)
	}
}

// CallTool routes a tool call to the appropriate backend.
func (gw *Mediator) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	if PapercupTools[name] {
		return gw.callPapercup(ctx, name, args)
	}

	gw.mu.RLock()
	rt, ok := gw.toolRoutes[name]
	gw.mu.RUnlock()

	if !ok {
		return &sdkmcp.CallToolResult{
			IsError: true,
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("unknown tool %q", name)}},
		}, nil
	}

	return gw.backends[rt.backendName].CallTool(ctx, name, args)
}

// callPapercup routes Papercup protocol tools using circuit_type and session affinity.
func (gw *Mediator) callPapercup(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	if name == "start_circuit" {
		return gw.routeStartCircuit(ctx, args)
	}

	// All other Papercup tools: route by session_id affinity.
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return errResult("session_id is required"), nil
	}

	gw.mu.RLock()
	backendName, ok := gw.sessionAffinity[sessionID]
	gw.mu.RUnlock()
	if !ok {
		return errResult(fmt.Sprintf("unknown session %q", sessionID)), nil
	}

	return gw.backends[backendName].CallTool(ctx, name, args)
}

// routeStartCircuit routes start_circuit by circuit_type.
func (gw *Mediator) routeStartCircuit(ctx context.Context, args map[string]any) (*sdkmcp.CallToolResult, error) {
	// Extract circuit_type from extra params.
	var circuitType string
	if extra, ok := args["extra"].(map[string]any); ok {
		circuitType, _ = extra["circuit_type"].(string)
	}

	// Resolve backend.
	gw.mu.RLock()
	var backendName string
	if circuitType != "" {
		backendName = gw.circuitBackends[circuitType]
	}
	if backendName == "" {
		backendName = gw.defaultBackend
	}
	gw.mu.RUnlock()

	if backendName == "" {
		return errResult(fmt.Sprintf("no backend for circuit_type %q", circuitType)), nil
	}

	rb, ok := gw.backends[backendName]
	if !ok {
		return errResult(fmt.Sprintf("backend %q not found", backendName)), nil
	}

	// Forward to backend.
	result, err := rb.CallTool(ctx, "start_circuit", args)
	if err != nil {
		return nil, err
	}

	// Parse session_id from response for affinity tracking.
	if sessionID := extractSessionID(result); sessionID != "" {
		gw.mu.Lock()
		gw.sessionAffinity[sessionID] = backendName
		gw.mu.Unlock()
		slog.Debug("session affinity registered",
			"session_id", sessionID,
			"backend", backendName,
			"circuit_type", circuitType,
		)
	}

	return result, nil
}

// extractSessionID parses session_id from a CallToolResult.
func extractSessionID(result *sdkmcp.CallToolResult) string {
	if result == nil || result.IsError {
		return ""
	}
	for _, c := range result.Content {
		tc, ok := c.(*sdkmcp.TextContent)
		if !ok {
			continue
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			continue
		}
		if sid, ok := out["session_id"].(string); ok {
			return sid
		}
	}
	return ""
}

func errResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}

// ListTools returns the merged tool list from all backends.
func (gw *Mediator) ListTools() []sdkmcp.Tool {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	seen := make(map[string]bool)
	var tools []sdkmcp.Tool

	// Papercup tools (deduplicated).
	for _, t := range gw.papercupSchemas {
		if !seen[t.Name] {
			tools = append(tools, t)
			seen[t.Name] = true
		}
	}

	// Non-Papercup tools.
	for _, rt := range gw.toolRoutes {
		if !seen[rt.tool.Name] {
			tools = append(tools, rt.tool)
			seen[rt.tool.Name] = true
		}
	}

	return tools
}

// Healthy returns true if all backends respond to ping.
func (gw *Mediator) Healthy(ctx context.Context) bool {
	for _, rb := range gw.backends {
		if !rb.Healthy(ctx) {
			return false
		}
	}
	return true
}

// UnhealthyBackends returns the names of backends that fail health checks.
func (gw *Mediator) UnhealthyBackends(ctx context.Context) []string {
	var unhealthy []string
	for name, rb := range gw.backends {
		if !rb.Healthy(ctx) {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}

// MCPServer creates an sdkmcp.Server that proxies all tool calls through the Mediator.
func (gw *Mediator) MCPServer() *sdkmcp.Server {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "origami-mediator", Version: "v0.1.0"},
		nil,
	)

	gw.mu.RLock()
	defer gw.mu.RUnlock()

	addHandler := func(t sdkmcp.Tool) {
		toolName := t.Name
		server.AddTool(
			&t,
			func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				var args map[string]any
				if req.Params.Arguments != nil {
					json.Unmarshal(req.Params.Arguments, &args)
				}
				return gw.CallTool(ctx, toolName, args)
			},
		)
	}

	// Register Papercup tools (once, routing handled by callPapercup).
	for _, t := range gw.papercupSchemas {
		addHandler(t)
	}

	// Register non-Papercup tools.
	for _, rt := range gw.toolRoutes {
		addHandler(rt.tool)
	}

	return server
}

// Handler returns an http.Handler with MCP, health, and readiness endpoints.
func (gw *Mediator) Handler() http.Handler {
	mcpServer := gw.MCPServer()

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return mcpServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if gw.Healthy(r.Context()) {
			w.WriteHeader(http.StatusOK)
			return
		}
		unhealthy := gw.UnhealthyBackends(r.Context())
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "unhealthy backends: %v", unhealthy)
	})
	return mux
}

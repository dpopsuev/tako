// Package mediator implements the Origami Mediator — a session-aware MCP
// router that coordinates schematics via the Papercup protocol. Papercup
// tools (circuit, signal, trace) are routed by circuit_type and session
// affinity. Non-Papercup tools are routed by tool name.
package mediator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/subprocess"
)

// Signal event name constants emitted by the Mediator.
const (
	EventRoute        = "route"
	EventSessionStart = "session_start"
	EventSessionDone  = "session_done"
)

// Signal meta key constants used in mediator routing signals.
const (
	MetaKeyBackend   = "backend"
	MetaKeySessionID = "session_id"
)

// PapercupTools enumerates the Papercup protocol tool names (consolidated).
// Note: "trace" is optional (only registered when StateDir is configured)
// and is routed as a non-Papercup tool via the standard tool routing table.
var PapercupTools = map[string]bool{
	"circuit": true,
	"signal":  true,
}

type routedTool struct {
	backendName string
	tool        sdkmcp.Tool
}

// BackendConfig describes a named backend MCP service.
type BackendConfig struct {
	Name        string
	Endpoint    string
	CircuitType string // if set, route circuit(action=start, circuit_type=X) to this backend
}

// Option configures a Mediator.
type Option func(*Mediator)

// WithStateDir sets the directory for trace recording. When set, each
// routing session writes a JSONL trace of routing decisions.
func WithStateDir(dir string) Option {
	return func(gw *Mediator) { gw.stateDir = dir }
}

// Mediator proxies MCP tool calls to backend services.
// Papercup tools (circuit, signal, trace) are routed by circuit_type
// (circuit action=start) and session affinity (all other Papercup calls).
// Non-Papercup tools are routed by tool name.
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

	// Observability: routing signals and optional trace recording.
	Bus      *agentport.MemBus
	stateDir string                // empty = tracing disabled
	recorder *engine.TraceRecorder // nil when tracing disabled

}

// New creates a Mediator that will connect to the given backends.
func New(configs []BackendConfig, opts ...Option) *Mediator {
	gw := &Mediator{
		backends:        make(map[string]*subprocess.RemoteBackend, len(configs)),
		sessions:        make(map[string]*sdkmcp.ClientSession, len(configs)),
		toolRoutes:      make(map[string]routedTool),
		circuitBackends: make(map[string]string),
		sessionAffinity: make(map[string]string),
		papercupSchemas: make(map[string]sdkmcp.Tool),
		Bus:             agentport.NewMemBus(),
	}
	for _, cfg := range configs {
		gw.backends[cfg.Name] = &subprocess.RemoteBackend{Endpoint: cfg.Endpoint}
		if cfg.CircuitType != "" {
			gw.circuitBackends[cfg.CircuitType] = cfg.Name
		} else {
			gw.defaultBackend = cfg.Name
		}
	}
	for _, opt := range opts {
		opt(gw)
	}
	return gw
}

// Start connects to all backends and builds the routing tables.
// If StateDir is configured, creates a trace recorder for routing decisions.
func (gw *Mediator) Start(ctx context.Context) error {
	// Set up trace recording if StateDir is configured.
	if gw.stateDir != "" {
		gw.initTraceRecording()
	}

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

func (gw *Mediator) initTraceRecording() {
	if err := os.MkdirAll(gw.stateDir, 0o755); err != nil {
		slog.WarnContext(context.Background(), circuit.LogMediatorStateDirFailed, slog.Any(circuit.LogKeyStateDir, gw.stateDir), slog.Any(circuit.LogKeyError, err))
		return
	}
	tracePath := filepath.Join(gw.stateDir, "mediator-trace.jsonl")
	rec, err := engine.NewTraceRecorder(tracePath)
	if err != nil {
		slog.WarnContext(context.Background(), circuit.LogMediatorTraceFailed, slog.Any(circuit.LogKeyError, err))
		return
	}
	gw.recorder = rec
	gw.Bus.OnEmit(func(sig agentport.Signal) {
		rec.HandleSignal(sig.Timestamp, sig.Event, sig.Agent, sig.CaseID, sig.Step, sig.Meta)
	})
}

// Stop closes all discovery sessions, backend connections, and the trace recorder.
func (gw *Mediator) Stop(ctx context.Context) {
	for _, s := range gw.sessions {
		s.Close()
	}
	for _, rb := range gw.backends {
		_ = rb.Stop(ctx)
	}
	if gw.recorder != nil {
		gw.recorder.Close()
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
	// For the consolidated "circuit" tool, check the action to determine routing.
	if name == "circuit" {
		action, _ := args["action"].(string)
		if action == "start" {
			return gw.routeStartCircuit(ctx, args)
		}
	}

	// All other Papercup tools: route by session_id affinity.
	sessionID, _ := args[circuit.ProtoKeySessionID].(string)
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

// routeStartCircuit routes circuit(action=start) by circuit_type.
func (gw *Mediator) routeStartCircuit(ctx context.Context, args map[string]any) (*sdkmcp.CallToolResult, error) {
	// Extract circuit_type from extra params.
	var circuitType string
	if extra, ok := args["extra"].(map[string]any); ok {
		circuitType, _ = extra[circuit.ExtraKeyCircuitType].(string)
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

	// Emit route agentport.
	gw.Bus.Emit(&agentport.Signal{
		Event: EventRoute,
		Agent: agentport.AgentMediator,
		Meta: map[string]string{
			MetaKeyBackend:              backendName,
			circuit.ExtraKeyCircuitType: circuitType,
		},
	})

	// Forward to backend.
	result, err := rb.CallTool(ctx, "circuit", args)
	if err != nil {
		return nil, err
	}

	// Parse session_id + alias from response for affinity tracking.
	if sessionID := extractSessionID(result); sessionID != "" {
		gw.mu.Lock()
		gw.sessionAffinity[sessionID] = backendName
		if alias := extractAlias(result); alias != "" {
			gw.sessionAffinity[alias] = backendName
		}
		gw.mu.Unlock()

		gw.Bus.Emit(&agentport.Signal{
			Event: EventSessionStart,
			Agent: agentport.AgentMediator,
			Meta: map[string]string{
				MetaKeySessionID:            sessionID,
				MetaKeyBackend:              backendName,
				circuit.ExtraKeyCircuitType: circuitType,
			},
		})

		slog.DebugContext(ctx, circuit.LogSessionAffinityRegistered, slog.Any(circuit.LogKeySessionID, sessionID), slog.Any(circuit.LogKeyBackend, backendName), slog.Any(circuit.LogKeyCircuitType, circuitType))
	}

	return result, nil
}

// NotifySessionDone emits a session_done signal for observability.
// Called externally when a child session completes (e.g., after get_report).
func (gw *Mediator) NotifySessionDone(sessionID, backendName string) {
	gw.Bus.Emit(&agentport.Signal{
		Event: EventSessionDone,
		Agent: agentport.AgentMediator,
		Meta: map[string]string{
			MetaKeySessionID: sessionID,
			MetaKeyBackend:   backendName,
		},
	})
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
		if sid, ok := out[circuit.ProtoKeySessionID].(string); ok {
			return sid
		}
	}
	return ""
}

// extractAlias parses alias from a CallToolResult (start response).
func extractAlias(result *sdkmcp.CallToolResult) string {
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
		if alias, ok := out["alias"].(string); ok {
			return alias
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
	for _, rt := range gw.toolRoutes { //nolint:gocritic // rangeValCopy: map value; unavoidable copy
		if !seen[rt.tool.Name] {
			tools = append(tools, rt.tool)
			seen[rt.tool.Name] = true
		}
	}

	return tools
}

// Signals returns the mediator's SignalBus for observability integration.
// MCP servers hosting the mediator can register get_signals on this bus.
func (gw *Mediator) Signals() *agentport.MemBus { return gw.Bus }

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
					_ = json.Unmarshal(req.Params.Arguments, &args)
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
	for _, rt := range gw.toolRoutes { //nolint:gocritic // rangeValCopy: map value; unavoidable copy
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

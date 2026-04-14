package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/battery/mcpserver"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/troupe/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const actionReport = "report"

// CircuitServer is a domain-agnostic MCP server that manages circuit
// sessions, capacity gating, worker prompt generation, and inline dispatch.
// Domain implementations create one by calling NewCircuitServer with a
// CircuitConfig that registers three hooks.
type CircuitServer struct {
	MCPServer *sdkmcp.Server
	Config    *CircuitConfig

	mu        sync.Mutex
	session   *CircuitSession
	sessCount int64

	defaultGetNextStepTimeout time.Duration
	defaultSessionTTL         time.Duration
}

// NewCircuitServer creates an MCP server with auto-registered circuit tools.
// The config provides domain hooks (session factory, step schemas, report
// formatter) while the server handles all protocol mechanics.
func NewCircuitServer(cfg *CircuitConfig) *CircuitServer {
	// Validate required fields — fail loudly, not silently at runtime.
	if cfg.CreateSession == nil {
		panic("CircuitConfig.CreateSession is required; start_circuit will panic without it")
	}
	if cfg.Name == "" {
		slog.WarnContext(context.Background(), circuit.LogConfigNameEmpty)
	}
	if len(cfg.StepSchemas) == 0 {
		slog.WarnContext(context.Background(), circuit.LogConfigSchemasEmpty)
	}
	if cfg.StateDir == "" {
		slog.WarnContext(context.Background(), circuit.LogConfigStateDirEmpty)
	}
	if len(cfg.StepSchemas) > 0 {
		names := make([]string, len(cfg.StepSchemas))
		for i, s := range cfg.StepSchemas {
			names[i] = s.Name
		}
		slog.InfoContext(context.Background(), circuit.LogStepSchemasRegistered, slog.Any(circuit.LogKeyNames, names), slog.Any(circuit.LogKeyCount, len(names)))
	}

	// Auto-wire observer to lifecycle callbacks. Consumer-set callbacks compose.
	if cfg.Observer != nil {
		wireObserverCallbacks(cfg)
	}

	fw := NewServer(cfg.Name, cfg.Version)

	getNextTimeout := 10 * time.Second
	if cfg.DefaultGetNextStepTimeout > 0 {
		getNextTimeout = time.Duration(cfg.DefaultGetNextStepTimeout) * time.Millisecond
	}
	sessionTTL := 5 * time.Minute
	if cfg.DefaultSessionTTL > 0 {
		sessionTTL = time.Duration(cfg.DefaultSessionTTL) * time.Millisecond
	}

	s := &CircuitServer{
		MCPServer:                 fw.MCPServer,
		Config:                    cfg,
		defaultGetNextStepTimeout: getNextTimeout,
		defaultSessionTTL:         sessionTTL,
	}
	s.registerTools()
	if cfg.StateDir != "" {
		s.registerTraceTools()
	}
	if cfg.PromptStore != nil {
		s.registerPromptTool()
	}
	if cfg.ResourceRegistry != nil && cfg.DomainFS != nil {
		s.registerResourceTool()
	}
	s.registerInstrumentTool()
	s.registerOperatorTool()
	s.registerBudgetTool()
	s.registerAgentTool()
	return s
}

func wireObserverCallbacks(cfg *CircuitConfig) {
	obs := cfg.Observer
	origDispatched := cfg.OnStepDispatched
	cfg.OnStepDispatched = func(caseID, step string) {
		obs.OnStepDispatched(caseID, step)
		if origDispatched != nil {
			origDispatched(caseID, step)
		}
	}
	origCompleted := cfg.OnStepCompleted
	cfg.OnStepCompleted = func(caseID, step string, dispatchID int64) {
		obs.OnStepCompleted(caseID, step, dispatchID)
		if origCompleted != nil {
			origCompleted(caseID, step, dispatchID)
		}
	}
	origDone := cfg.OnCircuitDone
	cfg.OnCircuitDone = func() {
		obs.OnCircuitDone()
		if origDone != nil {
			origDone()
		}
	}
	origEnd := cfg.OnSessionEnd
	cfg.OnSessionEnd = func() {
		obs.OnSessionEnd()
		if origEnd != nil {
			origEnd()
		}
	}
}

func setupTraceRecorder(stateDir, sessID string, bus *signal.MemBus, logger *slog.Logger) (recorder *engine.TraceRecorder, runDir string) {
	if stateDir == "" {
		return nil, ""
	}
	runDir = filepath.Join(stateDir, "runs", sessID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		logger.WarnContext(context.Background(), circuit.LogRunDirFailed, slog.Any(circuit.LogKeyRunDir, runDir), slog.Any(circuit.LogKeyError, err))
		return nil, runDir
	}
	var recErr error
	recorder, recErr = engine.NewTraceRecorder(filepath.Join(runDir, "trace.jsonl"))
	if recErr != nil {
		logger.WarnContext(context.Background(), circuit.LogTraceRecorderFailed, slog.Any(circuit.LogKeyError, recErr))
		return nil, runDir
	}
	bus.OnEmit(func(sig signal.Signal) {
		recorder.HandleSignal(sig.Timestamp, sig.Event, sig.Agent, sig.CaseID, sig.Step, sig.Meta)
	})
	return recorder, runDir
}

// --- Tool input/output types ---

// circuitInput is the unified input for the consolidated "circuit" tool.
type circuitInput struct {
	Action    string `json:"action"`               // start, step, submit, report, summary, detail, failing, weak, inspect, resume
	SessionID string `json:"session_id,omitempty"` // required for all actions except start
	// start params
	Parallel int            `json:"parallel,omitempty"`
	Force    bool           `json:"force,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
	Agent    string         `json:"agent,omitempty"`   // ACP agent name for auto-spawned workers (e.g., "cursor", "claude")
	Workers  int            `json:"workers,omitempty"` // number of ACP workers to auto-spawn (defaults to parallel)
	// step params
	TimeoutMS         int    `json:"timeout_ms,omitempty"`
	PreferredCaseID   string `json:"preferred_case_id,omitempty"`
	PreferredZone     string `json:"preferred_zone,omitempty"`
	Stickiness        int    `json:"stickiness,omitempty"`
	ConsecutiveMisses int    `json:"consecutive_misses,omitempty"`
	// submit params
	DispatchID int64          `json:"dispatch_id,omitempty"`
	Step       string         `json:"step,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
	// detail params
	CaseID string `json:"case_id,omitempty"`
	// weak params
	Threshold float64 `json:"threshold,omitempty"`
	// inspect/resume params
	WalkerID    string         `json:"walker_id,omitempty"`
	ResumeInput map[string]any `json:"resume_input,omitempty"`
	// confusion params
	Metric string `json:"metric,omitempty"` // field to analyze: component, category, defect_type, repos
	// diff params
	Against string `json:"against,omitempty"` // session ID to compare against (diff action)
}

// signalInput is the unified input for the consolidated "signal" tool.
type signalInput struct {
	Action    string            `json:"action"` // emit, list, health
	SessionID string            `json:"session_id"`
	Event     string            `json:"event,omitempty"`
	Agent     string            `json:"agent,omitempty"`
	CaseID    string            `json:"case_id,omitempty"`
	Step      string            `json:"step,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
	Since     int               `json:"since,omitempty"`
}

type emitSignalInput struct {
	SessionID string            `json:"session_id" jsonschema:"session ID from start_circuit"`
	Event     string            `json:"event" jsonschema:"signal event (dispatch, start, done, error, loop)"`
	Agent     string            `json:"agent" jsonschema:"agent type (main, sub, server)"`
	CaseID    string            `json:"case_id,omitempty" jsonschema:"case ID if applicable"`
	Step      string            `json:"step,omitempty" jsonschema:"circuit step if applicable"`
	Meta      map[string]string `json:"meta,omitempty" jsonschema:"optional key-value metadata"`
}

type emitSignalOutput struct {
	OK    string `json:"ok"`
	Index int    `json:"index"`
}

type getSignalsInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
	Since     int    `json:"since,omitempty" jsonschema:"return signals from this index onward (0-based)"`
}

type getSignalsOutput struct {
	Signals []signal.Signal `json:"signals"`
	Total   int             `json:"total"`
}

type getWorkerHealthInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
}

// HealthSummary is a stub for removed Supervisor health monitoring.
// TODO(troupe): replace with Troupe Hook-based health.
type HealthSummary struct {
	QueueDepth int              `json:"queue_depth"`
	Workers    []WorkerSnapshot `json:"workers"`
}

// WorkerSnapshot captures a point-in-time worker state.
type WorkerSnapshot struct {
	WorkerID   string `json:"worker_id"`
	State      string `json:"state"`
	ErrorCount int    `json:"error_count"`
	LastError  string `json:"last_error,omitempty"`
}

type getWorkerHealthOutput struct {
	HealthSummary
}

// --- Tool registration ---

func (s *CircuitServer) registerTools() {
	// Register consolidated "circuit" tool with action-based dispatch.
	s.MCPServer.AddTool(
		s.buildCircuitTool(),
		func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var input circuitInput
			if req.Params.Arguments != nil {
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return toolError(fmt.Errorf("invalid circuit arguments: %w", err)), nil
				}
			}
			return s.dispatchCircuitAction(ctx, req, &input)
		},
	)

	// Register consolidated "signal" tool with action-based dispatch.
	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "signal",
		Description: "Agent signal bus. Actions: emit (send signal), list (read signals), health (worker status).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleSignalDispatch))

	// Register approval gate tool if store is configured.
	s.registerApprovalTool()
}

// rawHandler adapts a typed handler into a raw ToolHandler by unmarshaling
// input from the request arguments. Replaces NoOutputSchema — no output
// schema is generated because we use the untyped AddTool path.
func rawHandler[In, Out any](h func(context.Context, *sdkmcp.CallToolRequest, In) (*sdkmcp.CallToolResult, Out, error)) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		var input In
		if req.Params != nil && req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
			}
		}
		res, out, err := h(ctx, req, input)
		if err != nil {
			return toolError(err), nil
		}
		// If the handler returned a CallToolResult, use it directly.
		// Otherwise, marshal the Out value as JSON (mirrors SDK typed handler behavior).
		if res != nil {
			return res, nil
		}
		return marshalToolResult(out)
	}
}

// dispatchCircuitAction routes the consolidated circuit tool to the appropriate handler.
//
//nolint:gocyclo,funlen // action dispatcher — one case per circuit action
func (s *CircuitServer) dispatchCircuitAction(ctx context.Context, req *sdkmcp.CallToolRequest, input *circuitInput) (*sdkmcp.CallToolResult, error) {
	switch input.Action {
	case "start":
		startInput := startCircuitInput{
			Parallel: input.Parallel,
			Force:    input.Force,
			Extra:    input.Extra,
			Agent:    input.Agent,
			Workers:  input.Workers,
		}
		res, out, err := s.handleStartCircuit(ctx, req, startInput)
		if err != nil {
			return toolError(err), nil
		}
		if res != nil {
			return res, nil
		}
		data, mErr := json.Marshal(out)
		if mErr != nil {
			return toolError(fmt.Errorf("marshal circuit start output: %w", mErr)), nil
		}
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
		}, nil

	case "step":
		stepInput := getNextStepInput{
			SessionID:         input.SessionID,
			TimeoutMS:         input.TimeoutMS,
			PreferredCaseID:   input.PreferredCaseID,
			PreferredZone:     input.PreferredZone,
			Stickiness:        input.Stickiness,
			ConsecutiveMisses: input.ConsecutiveMisses,
		}
		res, out, err := s.handleGetNextStep(ctx, req, stepInput)
		if err != nil {
			return toolError(err), nil
		}
		if res != nil {
			return res, nil
		}
		return marshalToolResult(out)

	case "submit":
		submitInput := submitStepInput{
			SessionID:  input.SessionID,
			DispatchID: input.DispatchID,
			Step:       input.Step,
			Fields:     input.Fields,
		}
		res, out, err := s.handleSubmitStep(ctx, req, submitInput)
		if err != nil {
			return toolError(err), nil
		}
		if res != nil {
			return res, nil
		}
		return marshalToolResult(out)

	case actionReport:
		reportInput := getReportInput{SessionID: input.SessionID}
		res, out, err := s.handleGetReport(ctx, req, reportInput)
		if err != nil {
			return toolError(err), nil
		}
		if res != nil {
			return res, nil
		}
		return marshalToolResult(out)

	case "summary":
		sess, err := s.getSession(input.SessionID)
		if err != nil {
			return toolError(err), nil
		}
		out, err := s.handleRunSummary(sess)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "detail":
		sess, err := s.getSession(input.SessionID)
		if err != nil {
			return toolError(err), nil
		}
		out, err := s.handleCaseDetail(sess, input.CaseID)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "failing":
		sess, err := s.getSession(input.SessionID)
		if err != nil {
			return toolError(err), nil
		}
		out, err := s.handleFailingMetrics(sess)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "weak":
		sess, err := s.getSession(input.SessionID)
		if err != nil {
			return toolError(err), nil
		}
		threshold := input.Threshold
		if threshold == 0 {
			threshold = 0.5
		}
		out, err := s.handleWeakCases(sess, threshold)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "confusion":
		sess, err := s.getSession(input.SessionID)
		if err != nil {
			return toolError(err), nil
		}
		out, err := s.handleConfusion(sess, input.Metric)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "diff":
		out, err := s.handleDiff(input.SessionID, input.Against)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "inspect":
		out, err := s.handleInspectCircuit(ctx, input)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	case "resume":
		out, err := s.handleResumeCircuit(ctx, input)
		if err != nil {
			return toolError(err), nil
		}
		return marshalToolResult(out)

	default:
		return toolError(fmt.Errorf("%w: %q; valid actions: start, step, submit, report, summary, detail, failing, weak, confusion, inspect, resume", ErrUnknownCircuitAction, input.Action)), nil
	}
}

// handleSignalDispatch routes the consolidated signal tool to the appropriate handler.
func (s *CircuitServer) handleSignalDispatch(ctx context.Context, req *sdkmcp.CallToolRequest, input *signalInput) (*sdkmcp.CallToolResult, any, error) {
	switch input.Action {
	case "emit":
		emitInput := emitSignalInput{
			SessionID: input.SessionID,
			Event:     input.Event,
			Agent:     input.Agent,
			CaseID:    input.CaseID,
			Step:      input.Step,
			Meta:      input.Meta,
		}
		return s.handleEmitSignal(ctx, req, &emitInput)

	case "list":
		listInput := getSignalsInput{
			SessionID: input.SessionID,
			Since:     input.Since,
		}
		return s.handleGetSignals(ctx, req, listInput)

	case "health":
		healthInput := getWorkerHealthInput{
			SessionID: input.SessionID,
		}
		return s.handleGetWorkerHealth(ctx, req, healthInput)

	default:
		return nil, nil, fmt.Errorf("%w: %q; valid actions: emit, list, health", ErrUnknownSignalAction, input.Action)
	}
}

// marshalToolResult marshals any value into a CallToolResult with JSON text content.
// Delegates to battery/mcpserver.JSONResult.
func marshalToolResult(v any) (*sdkmcp.CallToolResult, error) {
	res, err := mcpserver.JSONResult(v)
	if err != nil {
		return toolError(err), nil
	}
	return res, nil
}

// buildCircuitTool constructs the consolidated "circuit" Tool with an explicit
// InputSchema that includes the action field and domain-specific extra parameters
// from CircuitConfig.
func (s *CircuitServer) buildCircuitTool() *sdkmcp.Tool {
	extraProps := ""
	if len(s.Config.ExtraParamDefs) > 0 {
		var props []string
		for _, p := range s.Config.ExtraParamDefs {
			prop := fmt.Sprintf("%q:{\"type\":%q,\"description\":%q", p.Name, p.Type, p.Description)
			if len(p.Enum) > 0 {
				enumJSON, _ := json.Marshal(p.Enum)
				prop += fmt.Sprintf(",\"enum\":%s", enumJSON)
			}
			prop += "}"
			props = append(props, prop)
		}
		extraProps = strings.Join(props, ",")
	}

	extraSchema := `"additionalProperties":true,"description":"domain-specific parameters (for start action)","type":"object"`
	if extraProps != "" {
		extraSchema = fmt.Sprintf(`"additionalProperties":true,"description":"domain-specific parameters (for start action)","type":"object","properties":{%s}`, extraProps)
		var required []string
		for _, p := range s.Config.ExtraParamDefs {
			if p.Required {
				required = append(required, p.Name)
			}
		}
		if len(required) > 0 {
			reqJSON, _ := json.Marshal(required)
			extraSchema += fmt.Sprintf(`,"required":%s`, reqJSON)
		}
	}

	schema := json.RawMessage(fmt.Sprintf(
		`{"type":"object","properties":{"action":{"type":"string","description":"Action to perform","enum":["start","step","submit","report","summary","detail","failing","weak","confusion","diff"]},"session_id":{"type":"string","description":"session ID from circuit start"},"parallel":{"type":"integer","description":"number of parallel workers (start action)"},"force":{"type":"boolean","description":"cancel any existing session (start action)"},"extra":{%s},"timeout_ms":{"type":"integer","description":"max wait in milliseconds (step action)"},"preferred_case_id":{"type":"string","description":"prefer steps for this case ID (step action)"},"preferred_zone":{"type":"string","description":"prefer steps from this zone (step action)"},"stickiness":{"type":"integer","description":"zone stickiness level (step action)"},"consecutive_misses":{"type":"integer","description":"caller-tracked empty polls (step action)"},"dispatch_id":{"type":"integer","description":"dispatch ID from step (submit action)"},"step":{"type":"string","description":"circuit step name (submit action)"},"fields":{"type":"object","description":"artifact fields (submit action)"},"case_id":{"type":"string","description":"case ID (detail action)"},"threshold":{"type":"number","description":"convergence threshold (weak action, default 0.5)"},"metric":{"type":"string","description":"field to analyze (confusion action): component, category, defect_type"},"against":{"type":"string","description":"session ID to compare against (diff action)"}},"required":["action"]}`,
		extraSchema,
	))

	desc := "Circuit execution and analysis. Actions: start (begin run), step (get next prompt), submit (send artifact), report (full results), summary (compact metrics+cases), detail (single case drill-down), failing (failed metrics only), weak (low-convergence cases), confusion (group failures by misclassification pattern), diff (compare metrics between runs)."
	if len(s.Config.ExtraParamDefs) > 0 {
		var parts []string
		for _, p := range s.Config.ExtraParamDefs {
			entry := fmt.Sprintf("  %s (%s): %s", p.Name, p.Type, p.Description)
			if len(p.Enum) > 0 {
				entry += fmt.Sprintf(" [%s]", strings.Join(p.Enum, "|"))
			}
			if p.Required {
				entry += " (required)"
			}
			parts = append(parts, entry)
		}
		desc += "\n\nDomain parameters (pass in 'extra' for start action):\n" + strings.Join(parts, "\n")
	}

	return &sdkmcp.Tool{
		Name:        "circuit",
		Description: desc,
		InputSchema: schema,
	}
}

func (s *CircuitServer) handleEmitSignal(ctx context.Context, _ *sdkmcp.CallToolRequest, input *emitSignalInput) (*sdkmcp.CallToolResult, emitSignalOutput, error) { //nolint:unparam // handler pattern
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentSignalBus))
	if input.Event == "" {
		logger.WarnContext(ctx, circuit.LogEmitSignalNoEvent)
		return nil, emitSignalOutput{}, ErrEventRequired
	}
	if input.Agent == "" {
		logger.WarnContext(ctx, circuit.LogEmitSignalNoAgent)
		return nil, emitSignalOutput{}, ErrAgentRequired
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, emitSignalOutput{}, err
	}

	sess.Bus.Emit(&signal.Signal{
		Event:  input.Event,
		Agent:  input.Agent,
		CaseID: input.CaseID,
		Step:   input.Step,
		Meta:   input.Meta,
	})
	idx := sess.Bus.Len()

	if input.Event == signal.EventWorkerStarted {
		workerID := input.Meta[signal.MetaKeyWorkerID]
		mode := input.Meta[signal.MetaKeyMode]
		if workerID != "" {
			sess.RegisterWorker(workerID, mode)
			logger.DebugContext(ctx, circuit.LogWorkerRegistered, slog.Any(circuit.LogKeyWorkerID, workerID), slog.Any(circuit.LogKeyMode, mode))
		}
	}

	logger.DebugContext(ctx, circuit.LogSignalEmitted, slog.Any(circuit.LogKeyIndex, idx), slog.Any(circuit.LogKeyEvent, input.Event), slog.Any(circuit.LogKeyAgent, input.Agent))

	return nil, emitSignalOutput{
		OK:    "signal emitted",
		Index: idx,
	}, nil
}

func (s *CircuitServer) handleGetSignals(_ context.Context, _ *sdkmcp.CallToolRequest, input getSignalsInput) (*sdkmcp.CallToolResult, getSignalsOutput, error) { //nolint:unparam // handler pattern
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getSignalsOutput{}, err
	}

	signals := sess.Bus.Since(input.Since)
	return nil, getSignalsOutput{
		Signals: signals,
		Total:   sess.Bus.Len(),
	}, nil
}

func (s *CircuitServer) handleGetWorkerHealth(_ context.Context, _ *sdkmcp.CallToolRequest, input getWorkerHealthInput) (*sdkmcp.CallToolResult, getWorkerHealthOutput, error) { //nolint:unparam // handler pattern
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getWorkerHealthOutput{}, err
	}

	// TODO(troupe): full health monitoring via Hooks in Phase 3
	health := HealthSummary{
		QueueDepth: sess.dispatcher.ActiveDispatches(),
	}
	for id, mode := range sess.registeredWorkers {
		health.Workers = append(health.Workers, WorkerSnapshot{
			WorkerID: id,
			State:    mode,
		})
	}

	return nil, getWorkerHealthOutput{health}, nil
}

// --- Session management helpers ---

// SetSessionTTL configures the inactivity TTL on the current session.
func (s *CircuitServer) SetSessionTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.SetTTL(ttl)
	}
}

// SessionID returns the current session's ID, or empty string if none.
func (s *CircuitServer) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		return s.session.ID
	}
	return ""
}

// Shutdown cancels any active session.
func (s *CircuitServer) Shutdown() {
	s.mu.Lock()
	sess := s.session
	s.session = nil
	s.mu.Unlock()

	if sess != nil {
		sess.Cancel()
		<-sess.Done()
	}
}

// Session returns the current session for test introspection. Not for production use.
func (s *CircuitServer) Session() *CircuitSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

// toolError wraps a Go error into a CallToolResult with IsError=true.
// Delegates to battery/mcpserver.ErrorResult.
func toolError(err error) *sdkmcp.CallToolResult {
	return mcpserver.ErrorResult(err)
}

// Handler returns an http.Handler that serves the CircuitServer over
// Streamable HTTP MCP. Mounts /mcp (MCP protocol), /healthz, /readyz.
// Use with http.Server for production serving.
func (s *CircuitServer) Handler() http.Handler {
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return s.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func (s *CircuitServer) getSession(id string) (*CircuitSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil, ErrNoActiveSession
	}
	if s.session.ID != id && s.session.Alias != id {
		return nil, fmt.Errorf("%w: %q does not match active session %q (%s); the session may have been replaced or expired", ErrSessionId, id, s.session.ID, s.session.Alias)
	}
	return s.session, nil
}

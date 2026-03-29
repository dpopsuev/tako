package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"

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

func setupTraceRecorder(stateDir, sessID string, bus *agentport.MemBus, logger *slog.Logger) (recorder *engine.TraceRecorder, runDir string) {
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
	bus.OnEmit(func(sig agentport.Signal) {
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

// Legacy input types — kept for internal handler dispatch.
type startCircuitInput struct {
	Parallel int            `json:"parallel,omitempty" jsonschema:"number of parallel workers (default 1 = serial)"`
	Force    bool           `json:"force,omitempty" jsonschema:"cancel any existing session and start fresh"`
	Extra    map[string]any `json:"extra,omitempty" jsonschema:"domain-specific parameters"`
}

type startCircuitOutput struct {
	SessionID    string `json:"session_id"`
	Alias        string `json:"alias,omitempty"`
	TotalCases   int    `json:"total_cases"`
	Scenario     string `json:"scenario"`
	Status       string `json:"status"`
	WorkerPrompt string `json:"worker_prompt,omitempty"`
	WorkerCount  int    `json:"worker_count,omitempty"`
}

type getNextStepInput struct {
	SessionID         string `json:"session_id" jsonschema:"session ID from start_circuit"`
	TimeoutMS         int    `json:"timeout_ms,omitempty" jsonschema:"max wait in milliseconds (0 = block forever)"`
	PreferredCaseID   string `json:"preferred_case_id,omitempty" jsonschema:"prefer steps for this case ID"`
	PreferredZone     string `json:"preferred_zone,omitempty" jsonschema:"prefer steps from this zone/provider"`
	Stickiness        int    `json:"stickiness,omitempty" jsonschema:"zone stickiness level: any(0) slight(1) strong(2) exclusive(3)"`
	ConsecutiveMisses int    `json:"consecutive_misses,omitempty" jsonschema:"caller-tracked empty polls for work stealing"`
}

type getNextStepOutput struct {
	Done             bool   `json:"done"`
	Available        bool   `json:"available,omitempty"`
	Error            string `json:"error,omitempty"`
	CaseID           string `json:"case_id,omitempty"`
	Step             string `json:"step,omitempty"`
	PromptPath       string `json:"prompt_path,omitempty"`
	PromptContent    string `json:"prompt_content,omitempty"`
	ArtifactPath     string `json:"artifact_path,omitempty"`
	DispatchID       int64  `json:"dispatch_id,omitempty"`
	ActiveDispatches int    `json:"active_dispatches"`
	DesiredCapacity  int    `json:"desired_capacity"`
	CapacityWarning  string `json:"capacity_warning,omitempty"`
	ShouldStop       bool   `json:"should_stop,omitempty"`
}

type submitStepInput struct {
	SessionID  string         `json:"session_id" jsonschema:"session ID from start_circuit"`
	DispatchID int64          `json:"dispatch_id" jsonschema:"dispatch ID from get_next_step for artifact routing"`
	Step       string         `json:"step" jsonschema:"circuit step name (e.g. F0_RECALL, F1_TRIAGE)"`
	Fields     map[string]any `json:"fields" jsonschema:"artifact fields matching the step schema"`
}

type submitStepOutput struct {
	OK string `json:"ok"`
}

type getReportInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
}

type getReportOutput struct {
	Status     string `json:"status"`
	Report     string `json:"report,omitempty"`
	Structured any    `json:"structured,omitempty"`
	Error      string `json:"error,omitempty"`
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
	Signals []agentport.Signal `json:"signals"`
	Total   int                `json:"total"`
}

type getWorkerHealthInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
}

type getWorkerHealthOutput struct {
	agentport.HealthSummary
}

// --- Tool registration ---

// NoOutputSchema wraps a typed handler so the MCP SDK's Out type parameter is
// `any`, which suppresses outputSchema generation. Some MCP clients (including
// Cursor) don't support outputSchema and fail to parse the tools list when it's
// present.
func NoOutputSchema[In, Out any](h func(context.Context, *sdkmcp.CallToolRequest, In) (*sdkmcp.CallToolResult, Out, error)) sdkmcp.ToolHandlerFor[In, any] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input In) (*sdkmcp.CallToolResult, any, error) {
		res, out, err := h(ctx, req, input)
		return res, out, err
	}
}

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
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "signal",
		Description: "Agent signal bus. Actions: emit (send signal), list (read signals), health (worker status).",
	}, NoOutputSchema(s.handleSignalDispatch))
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
		return toolError(fmt.Errorf("%w: %q; valid actions: start, step, submit, report, summary, detail, failing, weak, inspect, resume", ErrUnknownCircuitAction, input.Action)), nil
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
func marshalToolResult(v any) (*sdkmcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return toolError(fmt.Errorf("marshal tool output: %w", err)), nil
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, nil
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
		`{"type":"object","properties":{"action":{"type":"string","description":"Action to perform","enum":["start","step","submit","report","summary","detail","failing","weak"]},"session_id":{"type":"string","description":"session ID from circuit start"},"parallel":{"type":"integer","description":"number of parallel workers (start action)"},"force":{"type":"boolean","description":"cancel any existing session (start action)"},"extra":{%s},"timeout_ms":{"type":"integer","description":"max wait in milliseconds (step action)"},"preferred_case_id":{"type":"string","description":"prefer steps for this case ID (step action)"},"preferred_zone":{"type":"string","description":"prefer steps from this zone (step action)"},"stickiness":{"type":"integer","description":"zone stickiness level (step action)"},"consecutive_misses":{"type":"integer","description":"caller-tracked empty polls (step action)"},"dispatch_id":{"type":"integer","description":"dispatch ID from step (submit action)"},"step":{"type":"string","description":"circuit step name (submit action)"},"fields":{"type":"object","description":"artifact fields (submit action)"},"case_id":{"type":"string","description":"case ID (detail action)"},"threshold":{"type":"number","description":"convergence threshold (weak action, default 0.5)"}},"required":["action"]}`,
		extraSchema,
	))

	desc := "Circuit execution and analysis. Actions: start (begin run), step (get next prompt), submit (send artifact), report (full results), summary (compact metrics+cases), detail (single case drill-down), failing (failed metrics only), weak (low-convergence cases)."
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

// --- Tool handlers ---

//nolint:funlen,unparam // session bootstrap with config validation + goroutine launch; handler pattern
func (s *CircuitServer) handleStartCircuit(ctx context.Context, _ *sdkmcp.CallToolRequest, input startCircuitInput) (*sdkmcp.CallToolResult, startCircuitOutput, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))
	s.mu.Lock()
	if s.session != nil {
		select {
		case <-s.session.Done():
			logger.InfoContext(ctx, circuit.LogReplacingSession, slog.Any(circuit.LogKeyOldID, s.session.ID))
			s.session.Cancel()
		default:
			if input.Force {
				logger.WarnContext(ctx, circuit.LogForceReplacingSession, slog.Any(circuit.LogKeyOldID, s.session.ID))
				s.session.Cancel()
			} else {
				s.mu.Unlock()
				return nil, startCircuitOutput{}, fmt.Errorf("%w: %s); pass force=true to replace it", ErrACircuitSessionIsAlreadyRunningId, s.session.ID)
			}
		}
		if s.Config.OnSessionEnd != nil {
			s.Config.OnSessionEnd()
		}
	}
	s.session = nil
	s.mu.Unlock()

	parallel := input.Parallel
	if parallel < 1 {
		parallel = 1
	}

	var runCtx context.Context
	var runCancel context.CancelFunc
	if s.Config.MaxSessionDuration > 0 {
		runCtx, runCancel = context.WithTimeout(context.Background(),
			time.Duration(s.Config.MaxSessionDuration)*time.Millisecond)
	} else {
		runCtx, runCancel = context.WithCancel(context.Background())
	}
	bus := agentport.NewMemBus()
	disp := dispatch.NewMuxDispatcher(runCtx, dispatch.WithMuxSignalBus(bus))

	// Generate session ID and set up trace recording BEFORE CreateSession
	// so the recorder is available as StartParams.Observer for walker events.
	s.mu.Lock()
	s.sessCount++
	seqN := s.sessCount
	s.mu.Unlock()
	sessID := fmt.Sprintf("s-%d-%d", time.Now().UnixMilli(), seqN)

	recorder, runDir := setupTraceRecorder(s.Config.StateDir, sessID, bus, logger)

	params := StartParams{
		Parallel: parallel,
		Force:    input.Force,
		Extra:    input.Extra,
		DomainFS: s.Config.DomainFS,
		StateDir: s.Config.StateDir,
	}
	if recorder != nil {
		params.Observer = recorder
	}

	if s.Config.Preflight != nil {
		if err := s.Config.Preflight(ctx); err != nil {
			runCancel()
			if recorder != nil {
				recorder.Close()
			}
			return nil, startCircuitOutput{}, fmt.Errorf("preflight failed: %w", err)
		}
	}

	startTime := time.Now()
	runFn, meta, err := s.Config.CreateSession(ctx, params, disp, bus)
	if err != nil {
		runCancel()
		if recorder != nil {
			recorder.Close()
		}
		paramSummary := fmt.Sprintf("parallel=%d, force=%v", parallel, input.Force)
		if len(input.Extra) > 0 {
			extraJSON, _ := json.Marshal(input.Extra)
			paramSummary += fmt.Sprintf(", extra=%s", extraJSON)
		}
		logger.ErrorContext(ctx, circuit.LogCircuitSessionFailed, slog.Any(circuit.LogKeyError, err.Error()), slog.Any(circuit.LogKeyParams, paramSummary), slog.Any(circuit.LogKeyElapsed, time.Since(startTime).Milliseconds()))
		return nil, startCircuitOutput{}, fmt.Errorf("create session (%s): %w", paramSummary, err)
	}

	sess := NewCircuitSession(runCtx, sessID, meta, parallel, disp, bus, runFn, runCancel)
	if alias, ok := input.Extra[ExtraKeySessionName].(string); ok {
		sess.Alias = alias
	}
	sess.recorder = recorder
	sess.runDir = runDir
	if tid, ok := input.Extra[circuit.ExtraKeyTraceID].(string); ok && tid != "" {
		sess.traceID = tid
	} else {
		sess.traceID = fmt.Sprintf("tr-%d", time.Now().UnixMilli())
	}
	sess.SetTTL(s.defaultSessionTTL)
	sess.Start() // launch run goroutine after all fields are set

	bus.Emit(&agentport.Signal{
		Event: EventSessionStarted,
		Agent: agentport.AgentServer,
		Meta: map[string]string{
			MetaKeyScenario:   meta.Scenario,
			MetaKeyTotalCases: fmt.Sprintf("%d", meta.TotalCases),
		},
	})

	s.mu.Lock()
	s.session = sess
	s.mu.Unlock()

	out := startCircuitOutput{
		SessionID:  sess.ID,
		Alias:      sess.Alias,
		TotalCases: sess.TotalCases,
		Scenario:   sess.Scenario,
		Status:     string(StateRunning),
	}
	if sess.DesiredCapacity > 1 {
		out.WorkerPrompt = sess.WorkerPrompt(s.Config)
		out.WorkerCount = sess.DesiredCapacity
	}

	logger.InfoContext(ctx, circuit.LogCircuitSessionStarted, slog.Any(circuit.LogKeySessionID, sess.ID), slog.Any(circuit.LogKeyScenario, sess.Scenario), slog.Any(circuit.LogKeyTotalCases, sess.TotalCases), slog.Any(circuit.LogKeyParallel, parallel), slog.Any(circuit.LogKeyElapsed, time.Since(startTime).Milliseconds()))

	return nil, out, nil
}

func (s *CircuitServer) handleGetNextStep(ctx context.Context, _ *sdkmcp.CallToolRequest, input getNextStepInput) (*sdkmcp.CallToolResult, getNextStepOutput, error) { //nolint:unparam // handler pattern: result may be non-nil in future actions
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getNextStepOutput{}, err
	}

	var timeout time.Duration
	if input.TimeoutMS > 0 {
		timeout = time.Duration(input.TimeoutMS) * time.Millisecond
	} else {
		timeout = s.defaultGetNextStepTimeout
	}

	hints := agentport.PullHints{
		PreferredCaseID:   input.PreferredCaseID,
		PreferredZone:     input.PreferredZone,
		Stickiness:        input.Stickiness,
		ConsecutiveMisses: input.ConsecutiveMisses,
	}

	sess.PullerEnter()
	dc, done, available, err := sess.GetNextStepWithHints(ctx, timeout, hints)
	sess.PullerExit()

	if err != nil {
		logger.WarnContext(ctx, circuit.LogGetNextStepError, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyError, err.Error()))
		return nil, getNextStepOutput{}, fmt.Errorf("get_next_step: %w", err)
	}

	if done {
		sess.SetGateExempt()
		out := getNextStepOutput{Done: true}
		if sessErr := sess.Err(); sessErr != nil {
			out.Error = sessErr.Error()
		}
		logger.InfoContext(ctx, circuit.LogCircuitComplete, slog.Any(circuit.LogKeySessionID, input.SessionID))
		sess.Bus.Emit(&agentport.Signal{Event: EventCircuitDone, Agent: agentport.AgentServer})
		if s.Config.OnCircuitDone != nil {
			s.Config.OnCircuitDone()
		}
		return nil, out, nil
	}

	if !available {
		sess.SetGateExempt()
		return nil, getNextStepOutput{Done: false, Available: false}, nil
	}

	logger.InfoContext(ctx, circuit.LogStepDispatched, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, dc.DispatchID))

	sess.Bus.Emit(&agentport.Signal{
		Event:  EventStepReady,
		Agent:  agentport.AgentServer,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta:   map[string]string{agentport.MetaKeyPromptPath: dc.PromptPath},
	})

	if s.Config.OnStepDispatched != nil {
		s.Config.OnStepDispatched(dc.CaseID, dc.Step)
	}

	sess.Supervisor.Process()
	inFlight := sess.AgentPull()
	desired := sess.DesiredCapacity
	out := getNextStepOutput{
		Done:             false,
		Available:        true,
		CaseID:           dc.CaseID,
		Step:             dc.Step,
		PromptPath:       dc.PromptPath,
		ArtifactPath:     dc.ArtifactPath,
		DispatchID:       dc.DispatchID,
		ActiveDispatches: inFlight,
		DesiredCapacity:  desired,
		ShouldStop:       sess.Supervisor.ShouldStop(),
	}

	if dc.PromptContent != "" {
		out.PromptContent = dc.PromptContent
	} else if dc.PromptPath != "" {
		if content, err := os.ReadFile(dc.PromptPath); err == nil {
			out.PromptContent = string(content)
		}
	}

	if desired > 1 && inFlight < desired {
		out.CapacityWarning = fmt.Sprintf(
			"system under capacity: %d/%d workers active",
			inFlight, desired)
		slog.DebugContext(ctx, circuit.LogUnderCapacity, slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession), slog.Any(circuit.LogKeyInFlight, inFlight), slog.Any(circuit.LogKeyDesired, desired), slog.Any(circuit.LogKeyDeficit, desired-inFlight))
	}

	return nil, out, nil
}

func (s *CircuitServer) handleSubmitStep(ctx context.Context, _ *sdkmcp.CallToolRequest, input submitStepInput) (*sdkmcp.CallToolResult, submitStepOutput, error) { //nolint:unparam // handler pattern
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))
	submitStart := time.Now()

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, submitStepOutput{}, err
	}

	if gateErr := sess.CheckCapacityGate(); gateErr != nil {
		logger.WarnContext(ctx, circuit.LogCapacityGateAdvisory, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyDispatchID, input.DispatchID), slog.Any(circuit.LogKeyDetail, gateErr.Error()))
	}

	if input.DispatchID == 0 {
		return nil, submitStepOutput{}, ErrDispatchIDRequired
	}

	if input.Step == "" {
		return nil, submitStepOutput{}, ErrStepRequired
	}

	schema, err := s.Config.FindSchema(input.Step)
	if err != nil {
		logger.WarnContext(ctx, circuit.LogStepSchemaFailed, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyStep, input.Step), slog.Any(circuit.LogKeyError, err.Error()))
		return nil, submitStepOutput{}, err
	}

	if err := schema.ValidateFields(input.Fields); err != nil {
		logger.WarnContext(ctx, circuit.LogStepSchemaFailed, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyStep, input.Step), slog.Any(circuit.LogKeyError, err.Error()))
		return nil, submitStepOutput{}, err
	}

	data, err := json.Marshal(input.Fields)
	if err != nil {
		return nil, submitStepOutput{}, fmt.Errorf("marshal fields: %w", err)
	}

	if err := sess.SubmitArtifact(ctx, input.DispatchID, data); err != nil {
		return nil, submitStepOutput{}, fmt.Errorf("submit_step: %w", err)
	}

	remaining := sess.AgentSubmit()
	sess.Bus.Emit(&agentport.Signal{
		Event: EventArtifactSubmitted,
		Agent: agentport.AgentServer,
		Step:  input.Step,
		Meta: map[string]string{
			agentport.MetaKeyBytes:    fmt.Sprintf("%d", len(data)),
			agentport.MetaKeyInFlight: fmt.Sprintf("%d", remaining),
			agentport.MetaKeyVia:      "submit_step",
		},
	})

	if s.Config.OnStepCompleted != nil {
		s.Config.OnStepCompleted("", input.Step, input.DispatchID)
	}

	logger.InfoContext(ctx, circuit.LogStepArtifactAccepted, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyDispatchID, input.DispatchID), slog.Any(circuit.LogKeyStep, input.Step), slog.Any(circuit.LogKeyElapsed, time.Since(submitStart).Milliseconds()))

	return nil, submitStepOutput{OK: "step accepted"}, nil
}

func (s *CircuitServer) handleGetReport(ctx context.Context, _ *sdkmcp.CallToolRequest, input getReportInput) (*sdkmcp.CallToolResult, getReportOutput, error) { //nolint:unparam // handler pattern
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getReportOutput{}, err
	}

	select {
	case <-sess.Done():
	case <-ctx.Done():
		return nil, getReportOutput{}, ctx.Err()
	}

	if sessErr := sess.Err(); sessErr != nil {
		logger.WarnContext(ctx, circuit.LogReportGeneratedError, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyStatus, string(StateError)))
		return nil, getReportOutput{
			Status: string(StateError),
			Error:  sessErr.Error(),
		}, nil
	}

	result := sess.Result()
	if result == nil {
		return nil, getReportOutput{Status: "no_report"}, nil
	}

	if s.Config.FormatReport == nil {
		logger.InfoContext(ctx, circuit.LogReportGenerated, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyStatus, string(StateDone)))
		return nil, getReportOutput{
			Status:     string(StateDone),
			Structured: result,
		}, nil
	}

	formatted, structured, err := s.Config.FormatReport(result)
	if err != nil {
		return nil, getReportOutput{
			Status: string(StateError),
			Error:  fmt.Sprintf("format report: %v", err),
		}, nil
	}

	logger.InfoContext(ctx, circuit.LogReportGenerated, slog.Any(circuit.LogKeySessionID, input.SessionID), slog.Any(circuit.LogKeyStatus, string(StateDone)))

	return nil, getReportOutput{
		Status:     string(StateDone),
		Report:     formatted,
		Structured: structured,
	}, nil
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

	sess.Bus.Emit(&agentport.Signal{
		Event:  input.Event,
		Agent:  input.Agent,
		CaseID: input.CaseID,
		Step:   input.Step,
		Meta:   input.Meta,
	})
	idx := sess.Bus.Len()

	if input.Event == agentport.EventWorkerStarted {
		workerID := input.Meta[agentport.MetaKeyWorkerID]
		mode := input.Meta[agentport.MetaKeyMode]
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

	sess.Supervisor.Process()
	health := sess.Supervisor.Health()
	health.QueueDepth = sess.dispatcher.ActiveDispatches()

	return nil, getWorkerHealthOutput{health}, nil
}

// --- HITL handlers ---

type inspectOutput struct {
	WalkerID      string               `json:"walker_id"`
	CurrentNode   string               `json:"current_node"`
	Status        string               `json:"status"`
	InterruptData map[string]any       `json:"interrupt_data,omitempty"`
	History       []circuit.StepRecord `json:"history"`
	LoopCounts    map[string]int       `json:"loop_counts,omitempty"`
}

type resumeOutput struct {
	Status string `json:"status"` // "resumed"
}

func (s *CircuitServer) handleInspectCircuit(ctx context.Context, input *circuitInput) (inspectOutput, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))

	if s.Config.Checkpointer == nil {
		return inspectOutput{}, ErrCheckpointerNotConfigured
	}
	if input.WalkerID == "" {
		return inspectOutput{}, ErrWalkerIDRequired
	}

	inspection, err := engine.InspectCheckpoint(s.Config.Checkpointer, input.WalkerID)
	if err != nil {
		return inspectOutput{}, err
	}

	logger.InfoContext(ctx, circuit.LogInspectCheckpoint,
		slog.Any(circuit.LogKeyWalkerID, input.WalkerID),
		slog.Any(circuit.LogKeyNode, inspection.CurrentNode),
		slog.Any(circuit.LogKeyStatus, inspection.Status))

	return inspectOutput{
		WalkerID:      inspection.WalkerID,
		CurrentNode:   inspection.CurrentNode,
		Status:        inspection.Status,
		InterruptData: inspection.InterruptData,
		History:       inspection.History,
		LoopCounts:    inspection.LoopCounts,
	}, nil
}

func (s *CircuitServer) handleResumeCircuit(ctx context.Context, input *circuitInput) (resumeOutput, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))

	if s.Config.Checkpointer == nil {
		return resumeOutput{}, ErrCheckpointerNotConfigured
	}
	if input.WalkerID == "" {
		return resumeOutput{}, ErrWalkerIDRequired
	}

	// Verify checkpoint exists and walker is interrupted.
	inspection, err := engine.InspectCheckpoint(s.Config.Checkpointer, input.WalkerID)
	if err != nil {
		return resumeOutput{}, err
	}
	if inspection.Status != "interrupted" {
		return resumeOutput{}, fmt.Errorf("%w: status is %q", engine.ErrWalkerNotInterrupted, inspection.Status)
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return resumeOutput{}, err
	}

	// Inject resume input and restart the walk via the session.
	if err := sess.ResumeWalk(s.Config.Checkpointer, input.WalkerID, input.ResumeInput); err != nil {
		return resumeOutput{}, fmt.Errorf("resume walk: %w", err)
	}

	logger.InfoContext(ctx, circuit.LogResumeWalk,
		slog.Any(circuit.LogKeySessionID, input.SessionID),
		slog.Any(circuit.LogKeyWalkerID, input.WalkerID))

	return resumeOutput{Status: "resumed"}, nil
}

// --- Post-mortem handlers ---

// handleRunSummary returns a compact summary of the circuit run result.
// Extracts metrics and per-case one-liners, excluding verbose fields like
// actual_rca_message, evidence_refs, evidence_gaps. Target: <4KB response.
func (s *CircuitServer) handleRunSummary(sess *CircuitSession) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	summary := make(map[string]any)

	// Extract metrics (compact)
	if metrics, ok := full["metrics"]; ok {
		summary["metrics"] = metrics
	}

	// Extract case one-liners
	if oneLiners := extractCaseOneLiners(full); len(oneLiners) > 0 {
		summary["cases"] = oneLiners
	}

	// If no structured data was extracted, return the full result as-is
	// (domain may not use metrics/case_results keys)
	if len(summary) == 0 {
		summary["result"] = full
	}

	return summary, nil
}

var caseOneLinerKeys = []string{"case_id", "defect_type", "category", "component", "convergence", "step_count"}

func extractCaseOneLiners(full map[string]any) []map[string]any {
	caseResults, ok := full["case_results"]
	if !ok {
		return nil
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return nil
	}
	var oneLiners []map[string]any
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		oneLiner := make(map[string]any)
		for _, key := range caseOneLinerKeys {
			if v, exists := cm[key]; exists {
				oneLiner[key] = v
			}
		}
		if len(oneLiner) > 0 {
			oneLiners = append(oneLiners, oneLiner)
		}
	}
	return oneLiners
}

// handleCaseDetail returns the full case_result for a single case_id.
func (s *CircuitServer) handleCaseDetail(sess *CircuitSession, caseID string) (any, error) {
	if caseID == "" {
		return nil, ErrCaseIdIsRequiredForDetailAction
	}
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	caseResults, ok := full["case_results"]
	if !ok {
		return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
	}
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cm["case_id"] == caseID {
			return cm, nil
		}
	}
	return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
}

// handleFailingMetrics returns only the metrics where pass=false.
func (s *CircuitServer) handleFailingMetrics(sess *CircuitSession) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	metricsRaw, ok := full["metrics"]
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metricsMap, ok := metricsRaw.(map[string]any)
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metricsList, ok := metricsMap["metrics"]
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metrics, ok := metricsList.([]any)
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}

	var failing []any
	for _, m := range metrics {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if pass, ok := mm["pass"].(bool); ok && !pass {
			failing = append(failing, mm)
		}
	}
	return map[string]any{"failing": failing}, nil
}

// handleWeakCases returns cases where convergence < threshold.
func (s *CircuitServer) handleWeakCases(sess *CircuitSession, threshold float64) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	caseResults, ok := full["case_results"]
	if !ok {
		return map[string]any{"weak": []any{}, "threshold": threshold}, nil
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return map[string]any{"weak": []any{}, "threshold": threshold}, nil
	}

	var weak []any
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		conv, ok := cm["convergence"].(float64)
		if ok && conv < threshold {
			weak = append(weak, cm)
		}
	}
	return map[string]any{"weak": weak, "threshold": threshold}, nil
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

// toolError wraps a Go error into a CallToolResult with IsError=true,
// matching how the SDK's typed handler converts errors.
func toolError(err error) *sdkmcp.CallToolResult {
	var res sdkmcp.CallToolResult
	res.SetError(err)
	return &res
}

func (s *CircuitServer) getSession(id string) (*CircuitSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil, ErrNoActiveSession
	}
	if s.session.ID != id {
		return nil, fmt.Errorf("%w: %q does not match active session %q; the session may have been replaced or expired", ErrSessionId, id, s.session.ID)
	}
	return s.session, nil
}

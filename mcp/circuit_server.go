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

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

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
func NewCircuitServer(cfg CircuitConfig) *CircuitServer {
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
		Config:                    &cfg,
		defaultGetNextStepTimeout: getNextTimeout,
		defaultSessionTTL:         sessionTTL,
	}
	s.registerTools()
	if cfg.StateDir != "" {
		s.registerTraceTools()
	}
	return s
}

// --- Tool input/output types ---

type startCircuitInput struct {
	Parallel int            `json:"parallel,omitempty" jsonschema:"number of parallel workers (default 1 = serial)"`
	Force    bool           `json:"force,omitempty" jsonschema:"cancel any existing session and start fresh"`
	Extra    map[string]any `json:"extra,omitempty" jsonschema:"domain-specific parameters"`
}

type startCircuitOutput struct {
	SessionID    string `json:"session_id"`
	TotalCases   int    `json:"total_cases"`
	Scenario     string `json:"scenario"`
	Status       string `json:"status"`
	WorkerPrompt string `json:"worker_prompt,omitempty"`
	WorkerCount  int    `json:"worker_count,omitempty"`
}

type getNextStepInput struct {
	SessionID       string `json:"session_id" jsonschema:"session ID from start_circuit"`
	TimeoutMS       int    `json:"timeout_ms,omitempty" jsonschema:"max wait in milliseconds (0 = block forever)"`
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
	Signals []dispatch.Signal `json:"signals"`
	Total   int               `json:"total"`
}

type getWorkerHealthInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_circuit"`
}

type getWorkerHealthOutput struct {
	dispatch.HealthSummary
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
	s.MCPServer.AddTool(
		s.buildStartCircuitTool(),
		func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var input startCircuitInput
			if req.Params.Arguments != nil {
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return toolError(fmt.Errorf("invalid start_circuit arguments: %w", err)), nil
				}
			}
			res, out, err := s.handleStartCircuit(ctx, req, input)
			if err != nil {
				return toolError(err), nil
			}
			if res != nil {
				return res, nil
			}
			data, mErr := json.Marshal(out)
			if mErr != nil {
				return toolError(fmt.Errorf("marshal start_circuit output: %w", mErr)), nil
			}
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
			}, nil
		},
	)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_next_step",
		Description: "Get the next circuit step prompt. Blocks until the runner is ready. Returns done=true when all cases are complete.",
	}, NoOutputSchema(s.handleGetNextStep))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "submit_step",
		Description: "Submit a schema-validated artifact for a circuit step. The step name selects the schema; fields are validated before routing.",
	}, NoOutputSchema(s.handleSubmitStep))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_report",
		Description: "Get the final circuit report with metrics and per-case results.",
	}, NoOutputSchema(s.handleGetReport))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "emit_signal",
		Description: "Emit a signal to the agent message bus for observability. Use to announce dispatch, start, done, error events.",
	}, NoOutputSchema(s.handleEmitSignal))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_signals",
		Description: "Read signals from the agent message bus. Returns all signals, or signals since a given index.",
	}, NoOutputSchema(s.handleGetSignals))

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_worker_health",
		Description: "Get worker health summary. Shows per-worker status, error counts, and replacement recommendations. The supervisor agent calls this to decide whether to replace or stop workers.",
	}, NoOutputSchema(s.handleGetWorkerHealth))
}

// buildStartCircuitTool constructs the start_circuit Tool with an explicit
// InputSchema that includes domain-specific extra parameters from CircuitConfig.
func (s *CircuitServer) buildStartCircuitTool() *sdkmcp.Tool {
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

	extraSchema := `"additionalProperties":true,"description":"domain-specific parameters","type":"object"`
	if extraProps != "" {
		extraSchema = fmt.Sprintf(`"additionalProperties":true,"description":"domain-specific parameters","type":"object","properties":{%s}`, extraProps)
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
		`{"type":"object","additionalProperties":false,"properties":{"parallel":{"type":"integer","description":"number of parallel workers (default 1 = serial)"},"force":{"type":"boolean","description":"cancel any existing session and start fresh"},"extra":{%s}}}`,
		extraSchema,
	))

	desc := "Start a circuit run. Spawns the runner goroutine and returns a session ID."
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
		desc += "\n\nDomain parameters (pass in 'extra'):\n" + strings.Join(parts, "\n")
	}

	return &sdkmcp.Tool{
		Name:        "start_circuit",
		Description: desc,
		InputSchema: schema,
	}
}

// --- Tool handlers ---

func (s *CircuitServer) handleStartCircuit(ctx context.Context, _ *sdkmcp.CallToolRequest, input startCircuitInput) (*sdkmcp.CallToolResult, startCircuitOutput, error) {
	logger := slog.Default().With("component", "circuit-session")
	s.mu.Lock()
	if s.session != nil {
		select {
		case <-s.session.Done():
			logger.Info("replacing completed/aborted session", "old_id", s.session.ID)
			s.session.Cancel()
		default:
			if input.Force {
				logger.Warn("force-replacing active session", "old_id", s.session.ID)
				s.session.Cancel()
			} else {
				s.mu.Unlock()
				return nil, startCircuitOutput{}, fmt.Errorf("a circuit session is already running (id=%s); pass force=true to replace it", s.session.ID)
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

	params := StartParams{
		Parallel: parallel,
		Force:    input.Force,
		Extra:    input.Extra,
	}

	var runCtx context.Context
	var runCancel context.CancelFunc
	if s.Config.MaxSessionDuration > 0 {
		runCtx, runCancel = context.WithTimeout(context.Background(),
			time.Duration(s.Config.MaxSessionDuration)*time.Millisecond)
	} else {
		runCtx, runCancel = context.WithCancel(context.Background())
	}
	bus := dispatch.NewSignalBus()
	disp := dispatch.NewMuxDispatcher(runCtx, dispatch.WithMuxSignalBus(bus))

	startTime := time.Now()
	runFn, meta, err := s.Config.CreateSession(ctx, params, disp, bus)
	if err != nil {
		runCancel()
		paramSummary := fmt.Sprintf("parallel=%d, force=%v", parallel, input.Force)
		if len(input.Extra) > 0 {
			extraJSON, _ := json.Marshal(input.Extra)
			paramSummary += fmt.Sprintf(", extra=%s", extraJSON)
		}
		logger.Error("circuit session failed",
			"error", err.Error(),
			"params", paramSummary,
			"elapsed_ms", time.Since(startTime).Milliseconds())
		return nil, startCircuitOutput{}, fmt.Errorf("create session (%s): %w", paramSummary, err)
	}

	s.mu.Lock()
	s.sessCount++
	seqN := s.sessCount
	s.mu.Unlock()
	sessID := fmt.Sprintf("s-%d-%d", time.Now().UnixMilli(), seqN)

	// Set up trace recording if StateDir is configured.
	var recorder *framework.TraceRecorder
	var runDir string
	if s.Config.StateDir != "" {
		runDir = filepath.Join(s.Config.StateDir, "runs", sessID)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			logger.Warn("failed to create run dir, tracing disabled",
				"run_dir", runDir, "error", err)
		} else {
			var recErr error
			recorder, recErr = framework.NewTraceRecorder(filepath.Join(runDir, "trace.jsonl"))
			if recErr != nil {
				logger.Warn("failed to create trace recorder",
					"error", recErr)
			} else {
				bus.SetOnEmit(func(sig dispatch.Signal) {
					recorder.HandleSignal(sig.Timestamp, sig.Event, sig.Agent, sig.CaseID, sig.Step, sig.Meta)
				})
			}
		}
	}

	sess := NewCircuitSession(runCtx, sessID, meta, parallel, disp, bus, runFn, runCancel)
	sess.recorder = recorder
	sess.runDir = runDir
	sess.SetTTL(s.defaultSessionTTL)

	bus.Emit("session_started", "server", "", "", map[string]string{
		"scenario":    meta.Scenario,
		"total_cases": fmt.Sprintf("%d", meta.TotalCases),
	})

	s.mu.Lock()
	s.session = sess
	s.mu.Unlock()

	out := startCircuitOutput{
		SessionID:  sess.ID,
		TotalCases: sess.TotalCases,
		Scenario:   sess.Scenario,
		Status:     string(StateRunning),
	}
	if sess.DesiredCapacity > 1 {
		out.WorkerPrompt = sess.WorkerPrompt(s.Config)
		out.WorkerCount = sess.DesiredCapacity
	}

	logger.Info("circuit session started",
		"session_id", sess.ID,
		"scenario", sess.Scenario,
		"total_cases", sess.TotalCases,
		"parallel", parallel,
		"elapsed_ms", time.Since(startTime).Milliseconds())

	return nil, out, nil
}

func (s *CircuitServer) handleGetNextStep(ctx context.Context, _ *sdkmcp.CallToolRequest, input getNextStepInput) (*sdkmcp.CallToolResult, getNextStepOutput, error) {
	logger := slog.Default().With("component", "circuit-session")
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

	hints := dispatch.PullHints{
		PreferredCaseID:   input.PreferredCaseID,
		PreferredZone:     input.PreferredZone,
		Stickiness:        input.Stickiness,
		ConsecutiveMisses: input.ConsecutiveMisses,
	}

	sess.PullerEnter()
	dc, done, available, err := sess.GetNextStepWithHints(ctx, timeout, hints)
	sess.PullerExit()

	if err != nil {
		logger.Warn("get_next_step error",
			"session_id", input.SessionID,
			"error", err.Error())
		return nil, getNextStepOutput{}, fmt.Errorf("get_next_step: %w", err)
	}

	if done {
		sess.SetGateExempt()
		out := getNextStepOutput{Done: true}
		if sessErr := sess.Err(); sessErr != nil {
			out.Error = sessErr.Error()
		}
		logger.Info("circuit complete", "session_id", input.SessionID)
		sess.Bus.Emit("circuit_done", "server", "", "", nil)
		if s.Config.OnCircuitDone != nil {
			s.Config.OnCircuitDone()
		}
		return nil, out, nil
	}

	if !available {
		sess.SetGateExempt()
		return nil, getNextStepOutput{Done: false, Available: false}, nil
	}

	logger.Info("step dispatched to worker",
		"session_id", input.SessionID,
		"case_id", dc.CaseID,
		"step", dc.Step,
		"dispatch_id", dc.DispatchID)

	sess.Bus.Emit("step_ready", "server", dc.CaseID, dc.Step, map[string]string{
		"prompt_path": dc.PromptPath,
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
		slog.Debug("under capacity", "component", "circuit-session",
			"in_flight", inFlight, "desired", desired, "deficit", desired-inFlight)
	}

	return nil, out, nil
}

func (s *CircuitServer) handleSubmitStep(ctx context.Context, _ *sdkmcp.CallToolRequest, input submitStepInput) (*sdkmcp.CallToolResult, submitStepOutput, error) {
	logger := slog.Default().With("component", "circuit-session")
	submitStart := time.Now()

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, submitStepOutput{}, err
	}

	if gateErr := sess.CheckCapacityGate(); gateErr != nil {
		logger.Warn("capacity gate advisory on submit_step",
			"session_id", input.SessionID, "dispatch_id", input.DispatchID, "detail", gateErr.Error())
	}

	if input.DispatchID == 0 {
		return nil, submitStepOutput{}, fmt.Errorf("dispatch_id is required (got 0); did you submit after available=false?")
	}

	if input.Step == "" {
		return nil, submitStepOutput{}, fmt.Errorf("step is required")
	}

	schema, err := s.Config.FindSchema(input.Step)
	if err != nil {
		logger.Warn("step schema validation failed",
			"session_id", input.SessionID, "step", input.Step, "error", err.Error())
		return nil, submitStepOutput{}, err
	}

	if err := schema.ValidateFields(input.Fields); err != nil {
		logger.Warn("step schema validation failed",
			"session_id", input.SessionID, "step", input.Step, "error", err.Error())
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
	sess.Bus.Emit("artifact_submitted", "server", "", input.Step, map[string]string{
		"bytes":     fmt.Sprintf("%d", len(data)),
		"in_flight": fmt.Sprintf("%d", remaining),
		"via":       "submit_step",
	})

	if s.Config.OnStepCompleted != nil {
		s.Config.OnStepCompleted("", input.Step, input.DispatchID)
	}

	logger.Info("step artifact accepted",
		"session_id", input.SessionID,
		"dispatch_id", input.DispatchID,
		"step", input.Step,
		"elapsed_ms", time.Since(submitStart).Milliseconds())

	return nil, submitStepOutput{OK: "step accepted"}, nil
}

func (s *CircuitServer) handleGetReport(ctx context.Context, _ *sdkmcp.CallToolRequest, input getReportInput) (*sdkmcp.CallToolResult, getReportOutput, error) {
	logger := slog.Default().With("component", "circuit-session")
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
		logger.Warn("report generated with error",
			"session_id", input.SessionID, "status", string(StateError))
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
		logger.Info("report generated",
			"session_id", input.SessionID, "status", string(StateDone))
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

	logger.Info("report generated",
		"session_id", input.SessionID, "status", string(StateDone))

	return nil, getReportOutput{
		Status:     string(StateDone),
		Report:     formatted,
		Structured: structured,
	}, nil
}

func (s *CircuitServer) handleEmitSignal(ctx context.Context, _ *sdkmcp.CallToolRequest, input emitSignalInput) (*sdkmcp.CallToolResult, emitSignalOutput, error) {
	logger := slog.Default().With("component", "signal-bus")
	if input.Event == "" {
		logger.Warn("emit_signal rejected: empty event field")
		return nil, emitSignalOutput{}, fmt.Errorf("event is required")
	}
	if input.Agent == "" {
		logger.Warn("emit_signal rejected: empty agent field")
		return nil, emitSignalOutput{}, fmt.Errorf("agent is required")
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, emitSignalOutput{}, err
	}

	sess.Bus.Emit(input.Event, input.Agent, input.CaseID, input.Step, input.Meta)
	idx := sess.Bus.Len()

	if input.Event == "worker_started" {
		workerID := input.Meta["worker_id"]
		mode := input.Meta["mode"]
		if workerID != "" {
			sess.RegisterWorker(workerID, mode)
			logger.Debug("worker registered", "worker_id", workerID, "mode", mode)
		}
	}

	logger.Debug("signal emitted", "index", idx, "event", input.Event, "agent", input.Agent)

	return nil, emitSignalOutput{
		OK:    "signal emitted",
		Index: idx,
	}, nil
}

func (s *CircuitServer) handleGetSignals(ctx context.Context, _ *sdkmcp.CallToolRequest, input getSignalsInput) (*sdkmcp.CallToolResult, getSignalsOutput, error) {
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

func (s *CircuitServer) handleGetWorkerHealth(_ context.Context, _ *sdkmcp.CallToolRequest, input getWorkerHealthInput) (*sdkmcp.CallToolResult, getWorkerHealthOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getWorkerHealthOutput{}, err
	}

	sess.Supervisor.Process()
	health := sess.Supervisor.Health()
	health.QueueDepth = sess.dispatcher.ActiveDispatches()

	return nil, getWorkerHealthOutput{HealthSummary: health}, nil
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
		return nil, fmt.Errorf("no active session; call start_circuit first to create one")
	}
	if s.session.ID != id {
		return nil, fmt.Errorf("session_id %q does not match active session %q; the session may have been replaced or expired", id, s.session.ID)
	}
	return s.session, nil
}

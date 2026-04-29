package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/dispatch"
	"github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/broker"
	"github.com/dpopsuev/tangle/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Core circuit protocol input/output types ---

// Legacy input types — kept for internal handler dispatch.
type startCircuitInput struct {
	Parallel int            `json:"parallel,omitempty" jsonschema:"number of parallel workers (default 1 = serial)"`
	Force    bool           `json:"force,omitempty" jsonschema:"cancel any existing session and start fresh"`
	Extra    map[string]any `json:"extra,omitempty" jsonschema:"domain-specific parameters"`
	Agent    string         `json:"agent,omitempty" jsonschema:"ACP agent name for auto-spawned workers (e.g. cursor, claude)"`
	Workers  int            `json:"workers,omitempty" jsonschema:"number of ACP workers to auto-spawn (defaults to parallel)"`
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

// --- Tool handlers ---

//nolint:funlen,unparam,gocyclo // session bootstrap with config validation + goroutine launch; handler pattern
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
	// Generate session ID early — needed for run directory path.
	s.mu.Lock()
	s.sessCount++
	seqN := s.sessCount
	s.mu.Unlock()
	sessID := fmt.Sprintf("s-%d-%d", time.Now().UnixMilli(), seqN)

	memBus := signal.NewMemBus()
	bus := signal.Bus(memBus)

	// When StateDir is configured, wrap MemBus with DurableEventLog so
	// events persist to {runDir}/events.jsonl alongside the trace.
	if s.Config.StateDir != "" {
		runDir := filepath.Join(s.Config.StateDir, "runs", sessID)
		if err := os.MkdirAll(runDir, 0o750); err == nil {
			eventsPath := filepath.Join(runDir, "events.jsonl")
			durableLog, durErr := signal.NewDurableJSONLines(eventsPath)
			if durErr != nil {
				logger.WarnContext(ctx, "durable event log failed, using in-memory only",
					slog.Any(circuit.LogKeyError, durErr))
			} else {
				// Wire MemBus emissions to durable store.
				memBus.OnEmit(func(sig signal.Signal) {
					ts, _ := time.Parse(time.RFC3339Nano, sig.Timestamp)
					durableLog.Emit(signal.Event{
						TraceID:   sig.CaseID,
						Source:    sig.Agent,
						Kind:      sig.Event,
						Timestamp: ts,
					})
				})
			}
		}
	}
	disp := dispatch.NewMuxDispatcher(runCtx, dispatch.WithMuxSignalBus(bus))

	recorder, runDir := setupTraceRecorder(s.Config.StateDir, sessID, memBus, logger)

	params := StartParams{
		Parallel:            parallel,
		Force:               input.Force,
		Extra:               input.Extra,
		DomainFS:            s.Config.DomainFS,
		StateDir:            s.Config.StateDir,
		PromptStore:         s.Config.PromptStore,
		ResourceRegistry:    s.Config.ResourceRegistry,
		SubCircuitResolvers: s.Config.SubCircuitResolvers,
		Tools:               s.Config.Tools,
		Manifests:           s.Config.Manifests,
		ApprovalStore:       s.Config.ApprovalStore,
	}
	// Compose observers: trace recorder + config-injected observers (telemetry, etc.).
	var observers []circuit.WalkObserver
	if recorder != nil {
		observers = append(observers, recorder)
	}
	observers = append(observers, s.Config.Observers...)
	if len(observers) == 1 {
		params.Observer = observers[0]
	} else if len(observers) > 1 {
		params.Observer = circuit.MultiObserver(observers)
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
	if alias, ok := input.Extra[ExtraKeySessionName].(string); ok && alias != "" {
		sess.Alias = alias
	} else {
		sess.Alias = GenerateHeraldicName()
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

	// Auto-spawn ACP workers when agent is specified.
	if input.Agent != "" {
		s.spawnACPWorkers(ctx, runCtx, input, parallel, disp, bus, logger)
	}

	bus.Emit(&signal.Signal{
		Event: EventSessionStarted,
		Agent: signal.AgentServer,
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

// spawnACPWorkers creates a Staff, spawns ACP agents, and launches an
// ACPWorkerDispatcher in a background goroutine.
func (s *CircuitServer) spawnACPWorkers(
	ctx, runCtx context.Context,
	input startCircuitInput,
	parallel int,
	disp *dispatch.MuxDispatcher,
	bus signal.Bus,
	logger *slog.Logger,
) {
	workerCount := input.Workers
	if workerCount < 1 {
		workerCount = parallel
	}
	if workerCount < 1 {
		workerCount = 1
	}
	// ACP launcher absorbed into Broker
	hook := newObservabilityHook(bus)
	broker := broker.New("", broker.WithHook(hook))
	for range workerCount {
		if _, spawnErr := broker.Spawn(runCtx, troupe.AgentConfig{
			Model: input.Agent,
			Role:  "worker",
		}); spawnErr != nil {
			logger.WarnContext(ctx, circuit.LogWorkerSpawnFailed,
				slog.Any(circuit.LogKeyAgent, input.Agent),
				slog.Any(circuit.LogKeyError, spawnErr))
		}
	}
	acpDisp := dispatch.NewACPWorkerDispatcher(
		disp, broker, "worker", workerCount,
		dispatch.WithACPWorkerLogger(logger),
		dispatch.WithACPWorkerBus(bus),
	)
	go func() {
		if acpErr := acpDisp.Run(runCtx); acpErr != nil {
			logger.ErrorContext(runCtx, circuit.LogACPDispatchError, slog.Any(circuit.LogKeyError, acpErr))
		}
	}()
	logger.InfoContext(ctx, circuit.LogWorkersSpawned,
		slog.Any(circuit.LogKeyAgent, input.Agent),
		slog.Any(circuit.LogKeyCount, workerCount))
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
		sess.Bus.Emit(&signal.Signal{Event: EventCircuitDone, Agent: signal.AgentServer})
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

	sess.Bus.Emit(&signal.Signal{
		Event:  EventStepReady,
		Agent:  signal.AgentServer,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta:   map[string]string{signal.MetaKeyPromptPath: dc.PromptPath},
	})

	if s.Config.OnStepDispatched != nil {
		s.Config.OnStepDispatched(dc.CaseID, dc.Step)
	}

	// TODO(troupe): health monitoring via Hooks
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
		ShouldStop:       false, // TODO(troupe): health monitoring via Hooks
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
	sess.Bus.Emit(&signal.Signal{
		Event: EventArtifactSubmitted,
		Agent: signal.AgentServer,
		Step:  input.Step,
		Meta: map[string]string{
			signal.MetaKeyBytes:    fmt.Sprintf("%d", len(data)),
			signal.MetaKeyInFlight: fmt.Sprintf("%d", remaining),
			signal.MetaKeyVia:      "submit_step",
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

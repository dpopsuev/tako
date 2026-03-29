package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
)

// SessionState tracks the lifecycle of a circuit session.
type SessionState string

const (
	StateRunning SessionState = "running"
	StateDone    SessionState = "done"
	StateError   SessionState = "error"
)

// CircuitSession holds the state for a single circuit run driven by MCP
// tool calls. It is domain-agnostic — domain logic lives in RunFunc and
// FormatReport, both provided via CircuitConfig.
type CircuitSession struct {
	ID              string
	Alias           string
	TotalCases      int
	Scenario        string
	DesiredCapacity int
	Bus             agentport.Bus

	log        *slog.Logger
	state      SessionState
	dispatcher *dispatch.MuxDispatcher
	result     any
	err        error
	doneCh     chan struct{}
	cancel     context.CancelFunc

	ttl            time.Duration
	lastActivityAt time.Time

	agentInFlight       int
	batchPeak           int
	sessionPeakInFlight int
	concurrentPullers   int
	peakPullers         int
	gateExempt          bool

	registeredWorkers map[string]string

	Supervisor *agentport.Supervisor

	recorder  *engine.TraceRecorder // nil when tracing disabled
	runDir    string                // {StateDir}/runs/{sessID}
	startedAt time.Time             // set when session is created
	traceID   string                // cross-circuit correlation ID

	runCtx context.Context // stored for deferred Start()
	runFn  RunFunc         // stored for deferred Start()

	mu sync.Mutex
}

// NewCircuitSession creates a circuit session but does NOT start the run
// goroutine. The caller must set recorder, runDir, and traceID on the
// returned session, then call Start() to launch the goroutine. This
// two-phase init prevents a data race between the goroutine reading
// those fields and the caller writing them.
func NewCircuitSession(
	ctx context.Context,
	id string,
	meta SessionMeta,
	parallel int,
	disp *dispatch.MuxDispatcher,
	bus agentport.Bus,
	runFn RunFunc,
	cancel context.CancelFunc,
) *CircuitSession {
	return &CircuitSession{
		ID:              id,
		log:             slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession)),
		state:           StateRunning,
		TotalCases:      meta.TotalCases,
		Scenario:        meta.Scenario,
		DesiredCapacity: parallel,
		Bus:             bus,
		dispatcher:      disp,
		doneCh:          make(chan struct{}),
		cancel:          cancel,
		Supervisor:      agentport.NewSupervisor(bus),
		startedAt:       time.Now(),
		runCtx:          ctx,
		runFn:           runFn,
	}
}

// Start launches the run goroutine. Must be called after setting
// recorder, runDir, and traceID on the session.
func (s *CircuitSession) Start() {
	go s.run(s.runCtx, s.runFn)
}

// GetState returns the current session state in a thread-safe manner.
func (s *CircuitSession) GetState() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// PullerEnter tracks a get_next_step call starting to block.
func (s *CircuitSession) PullerEnter() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.concurrentPullers++
	if s.concurrentPullers > s.peakPullers {
		s.peakPullers = s.concurrentPullers
	}
}

// PullerExit tracks a get_next_step call completing.
func (s *CircuitSession) PullerExit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.concurrentPullers > 0 {
		s.concurrentPullers--
	}
}

// AgentPull increments the in-flight counter on step delivery.
func (s *CircuitSession) AgentPull() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentInFlight++
	if s.agentInFlight > s.batchPeak {
		s.batchPeak = s.agentInFlight
	}
	if s.agentInFlight > s.sessionPeakInFlight {
		s.sessionPeakInFlight = s.agentInFlight
	}
	return s.agentInFlight
}

// AgentSubmit decrements the in-flight counter on artifact submission.
func (s *CircuitSession) AgentSubmit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentInFlight > 0 {
		s.agentInFlight--
	}
	if s.agentInFlight == 0 {
		s.batchPeak = 0
		s.gateExempt = false
	}
	return s.agentInFlight
}

// SetGateExempt marks the current batch as exempt from capacity gate.
func (s *CircuitSession) SetGateExempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gateExempt = true
}

// CheckCapacityGate returns an error if insufficient concurrent workers.
func (s *CircuitSession) CheckCapacityGate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.DesiredCapacity <= 1 || s.gateExempt {
		return nil
	}
	if s.batchPeak >= s.DesiredCapacity || s.sessionPeakInFlight >= s.DesiredCapacity || s.peakPullers >= s.DesiredCapacity {
		return nil
	}

	return fmt.Errorf(
		"%w: %d/%d concurrent workers observed (peak: %d, concurrent callers: %d). System expects %d workers",
		ErrCapacityGate, s.batchPeak, s.DesiredCapacity, s.sessionPeakInFlight, s.peakPullers, s.DesiredCapacity)
}

// AgentInFlight returns how many steps are pulled but not yet submitted.
func (s *CircuitSession) AgentInFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentInFlight
}

// RegisterWorker records a worker's declared mode from a worker_started agentport.
func (s *CircuitSession) RegisterWorker(id, mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.registeredWorkers == nil {
		s.registeredWorkers = make(map[string]string)
	}
	s.registeredWorkers[id] = mode
}

// WorkerModeStats returns the total registered workers and how many declared "stream" mode.
func (s *CircuitSession) WorkerModeStats() (total, stream int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	total = len(s.registeredWorkers)
	for _, mode := range s.registeredWorkers {
		if mode == "stream" {
			stream++
		}
	}
	return
}

// WorkerPrompt generates the complete worker loop instructions for v2
// choreography. The supervisor passes this verbatim to each Task subagent.
func (s *CircuitSession) WorkerPrompt(cfg *CircuitConfig) string {
	var sb strings.Builder

	preamble := cfg.WorkerPreamble
	if preamble == "" {
		preamble = fmt.Sprintf("You are a %s circuit worker.", cfg.Name)
	}
	sb.WriteString(preamble)
	sb.WriteString(" Process circuit steps by calling MCP tools directly in a loop until the circuit is drained.\n\n")

	if cfg.GatewayEndpoint != "" {
		sb.WriteString("## Connection\n\n")
		sb.WriteString(fmt.Sprintf("Connect to the MCP server at: %s\n\n", cfg.GatewayEndpoint))
	}

	sb.WriteString("## Protocol\n\nFollow this exact sequence:\n\n")

	sb.WriteString(fmt.Sprintf(`1. Emit start signal:
   signal(action="emit", session_id="%[1]s", event="worker_started", agent="worker",
          meta={"worker_id": "<unique_id>", "mode": "stream"})

2. Worker loop — repeat until done:
   response = circuit(action="step", session_id="%[1]s", timeout_ms=30000)

   if response.done → break
   if not response.available → retry immediately

   Read the prompt from response.prompt_content (full text inline).
   If prompt_content is empty, read from the file at response.prompt_path.

   Analyze the data in the prompt and produce the artifact fields
   matching the step schema below.

   circuit(action="submit", session_id="%[1]s",
           dispatch_id=response.dispatch_id,
           step=response.step,
           fields={<your artifact fields as a JSON object>})

3. Emit stop signal:
   signal(action="emit", session_id="%[1]s", event="worker_stopped", agent="worker",
          meta={"worker_id": "<unique_id>"})

`, s.ID))

	sb.WriteString("## Step schemas\n\n")
	if len(cfg.StepSchemas) > 0 {
		sb.WriteString("| Step | Required fields |\n|------|----------------|\n")
		for _, schema := range cfg.StepSchemas {
			fields := make([]string, 0, len(schema.Defs))
			for _, def := range schema.Defs {
				fields = append(fields, fmt.Sprintf("%s (%s)", def.Name, def.Type))
			}
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", schema.Name, strings.Join(fields, ", ")))
		}
	} else {
		sb.WriteString("(No step schemas defined — submit any valid JSON artifact.)\n")
	}

	sb.WriteString(`
## Rules

- Respond based ONLY on the prompt content provided.
- Do NOT read scenario files, ground truth, test code, or prior artifacts.
- Use circuit(action="submit") to submit structured fields.
- You call circuit(action="step") and circuit(action="submit") DIRECTLY. The parent does NOT relay for you.
- If available=false, retry immediately — the circuit may be between rounds.
- Process each step independently based on the prompt content.
`)

	return sb.String()
}

// SetTTL configures the session inactivity TTL and starts the watchdog.
func (s *CircuitSession) SetTTL(ttl time.Duration) {
	s.mu.Lock()
	s.ttl = ttl
	s.lastActivityAt = time.Now()
	s.mu.Unlock()

	go s.watchdog()
}

func (s *CircuitSession) touchActivity() {
	s.mu.Lock()
	prev := s.lastActivityAt
	s.lastActivityAt = time.Now()
	ttl := s.ttl
	s.mu.Unlock()

	if ttl > 0 && !prev.IsZero() {
		s.log.DebugContext(context.Background(), circuit.LogActivityReset, slog.Any(circuit.LogKeyGap, time.Since(prev)), slog.Any(circuit.LogKeyTTL, ttl))
	}
}

func (s *CircuitSession) watchdog() {
	s.mu.Lock()
	ttl := s.ttl
	s.mu.Unlock()

	if ttl <= 0 {
		return
	}

	ticker := time.NewTicker(ttl / 5)
	defer ticker.Stop()

	for {
		select {
		case <-s.doneCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			stale := time.Since(s.lastActivityAt)
			currentTTL := s.ttl
			s.mu.Unlock()

			if currentTTL <= 0 {
				return
			}

			if stale > currentTTL {
				s.log.WarnContext(context.Background(), circuit.LogTTLWatchdog, slog.Any(circuit.LogKeyStale, stale), slog.Any(circuit.LogKeyTTL, currentTTL), slog.Any(circuit.LogKeySessionID, s.ID))
				s.Bus.Emit(&agentport.Signal{
					Event: EventSessionError,
					Agent: agentport.AgentServer,
					Meta:  map[string]string{agentport.MetaKeyError: fmt.Sprintf("session TTL expired: no activity for %v", stale)},
				})
				s.dispatcher.Abort(fmt.Errorf("%w for %v", ErrSessionTTLExpired, stale))
				s.mu.Lock()
				s.state = StateError
				s.err = fmt.Errorf("%w for %v", ErrSessionTTLExpired, stale)
				s.mu.Unlock()
				s.cancel()
				return
			}
		}
	}
}

// Cancel terminates the runner goroutine.
func (s *CircuitSession) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Done returns a channel that closes when the runner exits.
func (s *CircuitSession) Done() <-chan struct{} {
	return s.doneCh
}

func (s *CircuitSession) run(ctx context.Context, runFn RunFunc) {
	defer close(s.doneCh)
	defer s.cancel()
	defer s.closeRecorder()

	result, err := runFn(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.state = StateError
		s.err = err
		s.Bus.Emit(&agentport.Signal{Event: EventSessionError, Agent: agentport.AgentServer, Meta: map[string]string{agentport.MetaKeyError: err.Error()}})
		s.log.ErrorContext(ctx, circuit.LogCircuitRunFailed, slog.Any(circuit.LogKeyError, err))
		s.writeReport(result) // write partial report even on error
		s.writeRunRecord()
		return
	}
	s.state = StateDone
	s.result = result
	s.Bus.Emit(&agentport.Signal{Event: EventSessionDone, Agent: agentport.AgentServer})
	s.log.InfoContext(ctx, circuit.LogCircuitRunComplete)
	s.writeReport(result)
	s.writeRunRecord()
}

// writeReport persists the run result as report.json if tracing is enabled.
func (s *CircuitSession) writeReport(result any) {
	if s.runDir == "" || result == nil {
		return
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.log.WarnContext(context.Background(), circuit.LogMarshalReportFailed, slog.Any(circuit.LogKeyError, err))
		return
	}
	path := filepath.Join(s.runDir, "report.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		s.log.WarnContext(context.Background(), circuit.LogWriteReportFailed, slog.Any(circuit.LogKeyPath, path), slog.Any(circuit.LogKeyError, err))
	} else {
		s.log.InfoContext(context.Background(), circuit.LogReportWritten, slog.Any(circuit.LogKeyPath, path))
	}
}

// writeRunRecord persists the run envelope as run.json alongside report.json.
func (s *CircuitSession) writeRunRecord() {
	if s.runDir == "" {
		return
	}

	now := time.Now()
	errCount := 0
	if s.err != nil {
		errCount = 1
	}

	traceEvents := 0
	if s.recorder != nil {
		traceEvents = s.recorder.EventCount()
	}

	rec := engine.RunRecord{
		ID:          s.ID,
		TraceID:     s.traceID,
		Scenario:    s.Scenario,
		Parallel:    s.DesiredCapacity,
		StartedAt:   s.startedAt,
		CompletedAt: now,
		DurationMs:  now.Sub(s.startedAt).Milliseconds(),
		CaseCount:   s.TotalCases,
		ErrorCount:  errCount,
		TraceEvents: traceEvents,
	}

	if err := engine.SaveRunRecord(s.runDir, &rec); err != nil {
		s.log.WarnContext(context.Background(), circuit.LogWriteRunFailed, slog.Any(circuit.LogKeyError, err))
	} else {
		s.log.InfoContext(context.Background(), circuit.LogRunRecordWritten, slog.Any(circuit.LogKeyPath, s.runDir))
	}
}

// closeRecorder flushes and closes the trace recorder.
func (s *CircuitSession) closeRecorder() {
	if s.recorder != nil {
		s.recorder.Close()
	}
}

// GetNextStep blocks until the runner produces the next prompt, the run
// completes, or the timeout expires.
// GetNextStep pulls the next step with no hints (FIFO).
func (s *CircuitSession) GetNextStep(ctx context.Context, timeout time.Duration) (dc agentport.Context, done, available bool, err error) {
	return s.GetNextStepWithHints(ctx, timeout, agentport.PullHints{})
}

// GetNextStepWithHints pulls the next step matching the given hints.
// Falls back based on stickiness. Blocks up to timeout.
func (s *CircuitSession) GetNextStepWithHints(ctx context.Context, timeout time.Duration, hints agentport.PullHints) (dc agentport.Context, done, available bool, err error) {
	select {
	case <-s.doneCh:
		return agentport.Context{}, true, false, nil
	default:
	}

	pullCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		pullCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	type pullResult struct {
		dc  agentport.Context
		err error
	}
	ch := make(chan pullResult, 1)
	go func() {
		dc, err := s.dispatcher.GetNextStepWithHints(pullCtx, hints)
		ch <- pullResult{dc, err}
	}()

	start := time.Now()

	select {
	case <-s.doneCh:
		if cancel != nil {
			cancel()
		}
		return agentport.Context{}, true, false, nil
	case r := <-ch:
		if r.err != nil {
			if errors.Is(pullCtx.Err(), context.DeadlineExceeded) {
				s.log.DebugContext(ctx, circuit.LogGetNextStepTimeout, slog.Any(circuit.LogKeyTimeout, timeout))
				return agentport.Context{}, false, false, nil
			}
			return agentport.Context{}, false, false, r.err
		}
		s.log.DebugContext(ctx, circuit.LogStepDelivered, slog.Any(circuit.LogKeyCaseID, r.dc.CaseID), slog.Any(circuit.LogKeyStep, r.dc.Step), slog.Any(circuit.LogKeyDispatchID, r.dc.DispatchID), slog.Any(circuit.LogKeyWait, time.Since(start)))
		return r.dc, false, true, nil
	}
}

// SubmitArtifact routes the agent's artifact to the correct Dispatch caller.
// Callers are responsible for ensuring data is valid JSON before calling this.
func (s *CircuitSession) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	s.touchActivity()
	return s.dispatcher.SubmitArtifact(ctx, dispatchID, data)
}

// Result returns the domain-specific run result, or nil if not done.
func (s *CircuitSession) Result() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// Err returns any error from the circuit run.
func (s *CircuitSession) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// CleanArtifactJSON strips markdown code fences that LLMs often wrap around
// JSON output (e.g. ```json\n{...}\n```).
func CleanArtifactJSON(data []byte) []byte {
	s := bytes.TrimSpace(data)
	if len(s) == 0 {
		return s
	}
	if bytes.HasPrefix(s, []byte("```")) {
		if idx := bytes.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		if bytes.HasSuffix(s, []byte("```")) {
			s = s[:len(s)-3]
		}
		s = bytes.TrimSpace(s)
	}
	return s
}

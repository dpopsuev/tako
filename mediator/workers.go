package mediator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Worker manager errors.
var (
	ErrWorkersAlreadyRunning = errors.New("workers are already running; call stop first")
	ErrNoWorkersRunning      = errors.New("no workers running")
	ErrUnknownWorkersAction  = errors.New("unknown workers action")
	ErrSessionRequired       = errors.New("session is required for workers start")
	ErrCircuitStepFailed     = errors.New("circuit step failed")
)

// workersInput is the MCP tool input for the workers tool.
type workersInput struct {
	Action  string `json:"action"`            // start, stop, health
	Session string `json:"session,omitempty"` // session ID or alias (start)
	Agent   string `json:"agent,omitempty"`   // agent CLI name (start, default: claude)
	Count   int    `json:"count,omitempty"`   // number of workers (start, default: 4)
}

// WorkerManager manages agent-worker goroutines that connect to the
// mediator's own gateway endpoint.
type WorkerManager struct {
	mu        sync.Mutex
	gateway   string // self-endpoint (e.g., "http://localhost:9000/mcp")
	cancel    context.CancelFunc
	running   bool
	count     int
	agent     string
	session   string
	done      chan struct{}
	completed atomic.Int64
	errored   atomic.Int64
}

// NewWorkerManager creates a manager that spawns workers connecting
// back to the given gateway endpoint.
func NewWorkerManager(gateway string) *WorkerManager {
	return &WorkerManager{gateway: gateway}
}

// Start spawns N agent-worker goroutines.
func (wm *WorkerManager) Start(ctx context.Context, session, agent string, count int) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.running {
		return ErrWorkersAlreadyRunning
	}

	if agent == "" {
		agent = "claude"
	}
	if count < 1 {
		count = 4
	}

	wm.session = session
	wm.agent = agent
	wm.count = count
	wm.running = true
	wm.completed.Store(0)
	wm.errored.Store(0)
	wm.done = make(chan struct{})

	workerCtx, cancel := context.WithCancel(ctx)
	wm.cancel = cancel

	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("worker-%d", id+1)
			if err := runMediatorWorker(workerCtx, wm.gateway, agent, session, name); err != nil {
				wm.errored.Add(1)
				slog.ErrorContext(workerCtx, "worker failed",
					slog.Any(circuit.LogKeyWorker, name),
					slog.Any(circuit.LogKeyError, err))
			} else {
				wm.completed.Add(1)
				slog.InfoContext(workerCtx, "worker done",
					slog.Any(circuit.LogKeyWorker, name))
			}
		}(i)
	}

	go func() {
		wg.Wait()
		wm.mu.Lock()
		wm.running = false
		wm.mu.Unlock()
		close(wm.done)
	}()

	slog.InfoContext(ctx, "workers started",
		slog.Any(circuit.LogKeySessionID, session),
		slog.Any(circuit.LogKeyAgent, agent),
		slog.Any(circuit.LogKeyCount, count))

	return nil
}

// Stop kills all running workers.
func (wm *WorkerManager) Stop() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if !wm.running {
		return ErrNoWorkersRunning
	}
	wm.cancel()
	return nil
}

// Health returns worker status.
func (wm *WorkerManager) Health() map[string]any {
	wm.mu.Lock()
	running := wm.running
	wm.mu.Unlock()

	return map[string]any{
		"running":   running,
		"session":   wm.session,
		"agent":     wm.agent,
		"count":     wm.count,
		"completed": wm.completed.Load(),
		"errored":   wm.errored.Load(),
	}
}

// runMediatorWorker is a single worker loop: spawn agent, connect to
// gateway, pull steps, pipe to agent, submit artifacts.
func runMediatorWorker(ctx context.Context, gateway, agentName, sessionID, workerName string) error {
	launcher := agentport.NewACPLauncher()
	staff := agentport.NewStaff(launcher)
	handle, err := staff.Spawn(ctx, "worker", agentport.LaunchConfig{
		Model: agentName,
		Role:  "worker",
	})
	if err != nil {
		return fmt.Errorf("spawn agent %q: %w", agentName, err)
	}
	defer staff.KillAll(ctx)

	transport := &sdkmcp.StreamableClientTransport{Endpoint: gateway}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-" + workerName, Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer session.Close()

	steps := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		nextResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "circuit",
			Arguments: mustMarshalMap(map[string]any{"action": "step", "session_id": sessionID, "timeout_ms": 30000}),
		})
		if err != nil {
			return fmt.Errorf("circuit/step: %w", err)
		}

		if nextResult.IsError {
			return fmt.Errorf("%w: %s", ErrCircuitStepFailed, extractTextContent(nextResult))
		}

		nextText := extractTextContent(nextResult)
		var step struct {
			Done       bool   `json:"done"`
			Available  bool   `json:"available"`
			Step       string `json:"step"`
			Prompt     string `json:"prompt_content"`
			DispatchID int64  `json:"dispatch_id"`
		}
		if err := json.Unmarshal([]byte(nextText), &step); err != nil {
			return fmt.Errorf("parse step: %w", err)
		}

		if step.Done {
			return nil
		}
		if !step.Available {
			continue
		}

		response, err := handle.Ask(ctx, step.Prompt)
		if err != nil {
			slog.ErrorContext(ctx, "agent ask failed",
				slog.Any(circuit.LogKeyWorker, workerName),
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyError, err))
			continue
		}

		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshalMap(map[string]any{
				"action":      "submit",
				"session_id":  sessionID,
				"dispatch_id": step.DispatchID,
				"step":        step.Step,
				"fields":      json.RawMessage(response),
			}),
		})
		if err != nil {
			slog.WarnContext(ctx, "submit failed",
				slog.Any(circuit.LogKeyWorker, workerName),
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyError, err))
		}
		steps++
	}
}

func mustMarshalMap(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func extractTextContent(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

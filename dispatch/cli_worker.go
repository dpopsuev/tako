package dispatch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// CLIWorkerDispatcher runs N independent worker goroutines, each pulling steps
// from a MuxDispatcher, piping the prompt to a CLI command, and submitting the
// stdout as an artifact. This gives CLI-based LLM tools full Papercup bus
// power without requiring the CLI tool to know about the protocol.
type CLIWorkerDispatcher struct {
	mux     *MuxDispatcher
	bus     agentport.Bus
	command string
	args    []string
	workers int
	timeout time.Duration
	log     *slog.Logger
}

// CLIWorkerOption configures a CLIWorkerDispatcher.
type CLIWorkerOption func(*CLIWorkerDispatcher)

// WithCLIWorkerArgs sets additional arguments passed to the CLI command.
func WithCLIWorkerArgs(args ...string) CLIWorkerOption {
	return func(d *CLIWorkerDispatcher) { d.args = args }
}

// WithCLIWorkerTimeout sets the maximum execution time for a single CLI invocation.
func WithCLIWorkerTimeout(t time.Duration) CLIWorkerOption {
	return func(d *CLIWorkerDispatcher) { d.timeout = t }
}

// WithCLIWorkerLogger sets a structured logger.
func WithCLIWorkerLogger(l *slog.Logger) CLIWorkerOption {
	return func(d *CLIWorkerDispatcher) { d.log = l }
}

// WithCLIWorkerBus attaches a agentport.Bus for worker lifecycle signals.
func WithCLIWorkerBus(bus agentport.Bus) CLIWorkerOption {
	return func(d *CLIWorkerDispatcher) { d.bus = bus }
}

// NewCLIWorkerDispatcher creates a dispatcher that runs N worker goroutines.
// Each worker independently pulls from the MuxDispatcher, invokes the CLI
// command with the prompt on stdin, and submits the stdout as the artifact.
//
// The command path is validated at construction time.
func NewCLIWorkerDispatcher(mux *MuxDispatcher, command string, workers int, opts ...CLIWorkerOption) (*CLIWorkerDispatcher, error) {
	resolved, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("dispatch/cli_worker: command %q not found in PATH: %w", command, err)
	}

	if workers < 1 {
		workers = 1
	}

	d := &CLIWorkerDispatcher{
		mux:     mux,
		command: resolved,
		workers: workers,
		timeout: 5 * time.Minute,
		log:     discardLogger(),
	}
	for _, o := range opts {
		o(d)
	}
	return d, nil
}

// Run starts N worker goroutines and blocks until all complete (the
// MuxDispatcher is closed or its context is canceled). Each worker runs
// the Papercup v2 competing-consumer loop independently.
func (d *CLIWorkerDispatcher) Run(ctx context.Context) error {
	return runWorkers(ctx, d.workers, "cli-worker", d.workerLoop)
}

// runWorkers starts N goroutines and blocks until all complete. Each goroutine
// receives a unique workerID (prefix-N) and runs the provided loop function.
func runWorkers(ctx context.Context, n int, prefix string, loop func(ctx context.Context, workerID string) error) error {
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("%s-%d", prefix, i)
		go func() {
			defer wg.Done()
			if err := loop(ctx, workerID); err != nil {
				errs <- fmt.Errorf("%s: %w", workerID, err)
			}
		}()
	}

	wg.Wait()
	close(errs)

	var firstErr error
	for err := range errs {
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *CLIWorkerDispatcher) workerLoop(ctx context.Context, workerID string) error {
	d.emit(agentport.EventWorkerStarted, "", "", map[string]string{agentport.MetaKeyWorkerID: workerID})
	defer d.emit(agentport.EventWorkerStopped, "", "", map[string]string{agentport.MetaKeyWorkerID: workerID})

	for {
		dc, err := d.mux.GetNextStep(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("get_next_step: %w", err)
		}

		d.emit(agentport.EventWorkerStart, dc.CaseID, dc.Step, map[string]string{agentport.MetaKeyWorkerID: workerID})

		artifact, err := d.execCLI(ctx, dc)
		if err != nil {
			d.emit(agentport.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				agentport.MetaKeyWorkerID: workerID,
				agentport.MetaKeyError:    err.Error(),
			})
			d.log.ErrorContext(ctx, circuit.LogCLIExecFailed, slog.Any(circuit.LogKeyWorkerID, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyError, err.Error()))
			continue
		}

		if err := d.mux.SubmitArtifact(ctx, dc.DispatchID, artifact); err != nil {
			d.emit(agentport.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				agentport.MetaKeyWorkerID: workerID,
				agentport.MetaKeyError:    err.Error(),
			})
			return fmt.Errorf("submit_artifact dispatch_id=%d: %w", dc.DispatchID, err)
		}

		d.emit(agentport.EventWorkerDone, dc.CaseID, dc.Step, map[string]string{
			agentport.MetaKeyWorkerID: workerID,
			agentport.MetaKeyBytes:    fmt.Sprintf("%d", len(artifact)),
		})

		d.log.InfoContext(ctx, circuit.LogStepComplete, slog.Any(circuit.LogKeyWorkerID, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyBytes, len(artifact)))
	}
}

func (d *CLIWorkerDispatcher) execCLI(ctx context.Context, dc agentport.Context) ([]byte, error) {
	prompt, err := os.ReadFile(dc.PromptPath)
	if err != nil {
		return nil, fmt.Errorf("read prompt: %w", err)
	}

	execCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	args := make([]string, len(d.args))
	copy(args, d.args)

	cmd := exec.CommandContext(execCtx, d.command, args...) //nolint:gosec // command is from trusted CLIWorkerDispatcher config
	cmd.Stdin = bytes.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w after %v (stderr: %s)", ErrCommandTimedOut, d.timeout, stderrStr)
		}
		return nil, fmt.Errorf("command failed: %w (stderr: %s)", err, stderrStr)
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		return nil, fmt.Errorf("%w (stderr: %s)", ErrCommandNoOutput, stderr.String())
	}

	d.log.DebugContext(ctx, circuit.LogCLIExec, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyBytes, len(output)), slog.Any(circuit.LogKeyElapsedDur, time.Since(start)))

	return output, nil
}

func (d *CLIWorkerDispatcher) emit(event, caseID, step string, meta map[string]string) {
	if d.bus != nil {
		d.bus.Emit(&agentport.Signal{
			Event:  event,
			Agent:  agentport.AgentWorker,
			CaseID: caseID,
			Step:   step,
			Meta:   meta,
		})
	}
}

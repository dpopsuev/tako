package dispatch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/dpopsuev/origami/agentport"
)

const (
	logKeyCLICommand      = "command"
	logKeyCLICaseID       = "case_id"
	logKeyCLIStep         = "step"
	logKeyCLIResponseSize = "response_bytes"
	logKeyCLIElapsed      = "elapsed"
)

var (
	ErrCLITimeout  = errors.New("dispatch/cli: command timed out")
	ErrCLINoOutput = errors.New("dispatch/cli: command produced no output")
)

// CLIDispatcher shells out to an external CLI tool to process a prompt.
type CLIDispatcher struct {
	Command string
	Args    []string
	Timeout time.Duration
	Logger  *slog.Logger
}

// CLIOption configures a CLIDispatcher.
type CLIOption func(*CLIDispatcher)

func WithCLIArgs(args ...string) CLIOption     { return func(d *CLIDispatcher) { d.Args = args } }
func WithCLITimeout(t time.Duration) CLIOption { return func(d *CLIDispatcher) { d.Timeout = t } }
func WithCLILogger(l *slog.Logger) CLIOption   { return func(d *CLIDispatcher) { d.Logger = l } }

// NewCLIDispatcher creates a dispatcher that invokes the given command.
func NewCLIDispatcher(command string, opts ...CLIOption) (*CLIDispatcher, error) {
	resolved, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("dispatch/cli: command %q not found in PATH: %w", command, err)
	}
	d := &CLIDispatcher{
		Command: resolved,
		Timeout: 5 * time.Minute,
		Logger:  agentport.DiscardLogger(),
	}
	for _, o := range opts {
		o(d)
	}
	return d, nil
}

// Dispatch reads the prompt, pipes it to the CLI, captures stdout as artifact.
func (d *CLIDispatcher) Dispatch(ctx context.Context, dctx agentport.Context) ([]byte, error) {
	prompt, err := os.ReadFile(dctx.PromptPath)
	if err != nil {
		return nil, fmt.Errorf("dispatch/cli: read prompt: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()

	args := make([]string, len(d.Args))
	copy(args, d.Args)

	cmd := exec.CommandContext(cmdCtx, d.Command, args...) //nolint:gosec // validated via LookPath
	cmd.Stdin = bytes.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	d.Logger.InfoContext(ctx, "dispatching CLI command",
		slog.String(logKeyCLICommand, d.Command),
		slog.String(logKeyCLICaseID, dctx.CaseID),
		slog.String(logKeyCLIStep, dctx.Step),
	)

	start := time.Now()
	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w after %v (stderr: %s)", ErrCLITimeout, d.Timeout, stderrStr)
		}
		return nil, fmt.Errorf("dispatch/cli: command failed: %w (stderr: %s)", err, stderrStr)
	}

	output := stdout.Bytes()
	elapsed := time.Since(start)

	if len(output) == 0 {
		return nil, fmt.Errorf("%w (stderr: %s)", ErrCLINoOutput, stderr.String())
	}

	if err := os.WriteFile(dctx.ArtifactPath, output, 0o600); err != nil {
		return nil, fmt.Errorf("dispatch/cli: write artifact: %w", err)
	}

	d.Logger.InfoContext(ctx, "CLI dispatch complete",
		slog.String(logKeyCLICaseID, dctx.CaseID),
		slog.String(logKeyCLIStep, dctx.Step),
		slog.Int(logKeyCLIResponseSize, len(output)),
		slog.Duration(logKeyCLIElapsed, elapsed),
	)

	return output, nil
}

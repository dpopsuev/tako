// Package cli provides a connector for running Origami circuits from
// stdin/arguments in headless mode. It reads JSON input, invokes the
// engine, and writes captured artifacts as JSON to the configured output.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// CLIRunner executes circuits in headless mode, reading JSON input and
// writing walk artifacts to an output writer.
type CLIRunner struct {
	circuitPath string
	opts        []engine.RunOption
	output      io.Writer
	logger      *slog.Logger
}

// Option configures a CLIRunner.
type Option func(*CLIRunner)

// WithOutput sets the writer for walk result output. Defaults to os.Stdout.
func WithOutput(w io.Writer) Option {
	return func(r *CLIRunner) { r.output = w }
}

// WithRunOptions appends engine.RunOptions for the walk invocation.
func WithRunOptions(opts ...engine.RunOption) Option {
	return func(r *CLIRunner) { r.opts = append(r.opts, opts...) }
}

// WithLogger sets the logger for CLI runner operations.
func WithLogger(l *slog.Logger) Option {
	return func(r *CLIRunner) { r.logger = l }
}

// NewCLIRunner creates a CLIRunner for the given circuit path.
func NewCLIRunner(circuitPath string, opts ...Option) *CLIRunner {
	r := &CLIRunner{
		circuitPath: circuitPath,
		output:      os.Stdout,
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RunFromStdin reads JSON input from os.Stdin and runs the circuit.
func (r *CLIRunner) RunFromStdin(ctx context.Context) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("cli: read stdin: %w", err)
	}

	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("cli: unmarshal stdin: %w", err)
	}

	return r.RunWithInput(ctx, input)
}

// RunWithInput runs the circuit with the given input and writes captured
// artifacts as JSON to the configured output.
func (r *CLIRunner) RunWithInput(ctx context.Context, input any) error {
	r.logger.InfoContext(ctx, "cli run start",
		slog.String(circuit.LogKeyCircuit, r.circuitPath),
	)

	obs, capture := engine.NewCapture()
	opts := make([]engine.RunOption, 0, len(r.opts)+2)
	opts = append(opts, r.opts...)
	opts = append(opts, engine.WithOutputCapture(obs), engine.WithLogger(r.logger))

	if err := engine.Run(ctx, r.circuitPath, input, opts...); err != nil {
		return fmt.Errorf("cli: run circuit: %w", err)
	}

	artifacts := capture.Artifacts()
	if len(artifacts) == 0 {
		r.logger.InfoContext(ctx, "cli run complete, no artifacts",
			slog.String(circuit.LogKeyCircuit, r.circuitPath),
		)
		return nil
	}

	out := make(map[string]any, len(artifacts))
	for node, a := range artifacts {
		out[node] = a.Raw()
	}

	enc := json.NewEncoder(r.output)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("cli: encode output: %w", err)
	}

	r.logger.InfoContext(ctx, "cli run complete",
		slog.String(circuit.LogKeyCircuit, r.circuitPath),
		slog.Int(circuit.LogKeyCount, len(artifacts)),
	)
	return nil
}

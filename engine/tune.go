package engine

// Category: Execution — instrument preflight verification.

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"

	"github.com/dpopsuev/origami/circuit"
)

// TuneAll runs the preflight tune command for every instrument in the registry.
// It fails fast on the first failure. Instruments are tuned in sorted order
// for deterministic output.
func TuneAll(ctx context.Context, instruments InstrumentRegistry, workDir string) error {
	if len(instruments) == 0 {
		return nil
	}

	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentInstrument))

	names := make([]string, 0, len(instruments))
	for name := range instruments {
		names = append(names, name)
	}
	sort.Strings(names)

	logger.InfoContext(ctx, circuit.LogTuneAllStarted,
		slog.Any(circuit.LogKeyCount, len(names)))

	for _, name := range names {
		manifest := instruments[name]
		if err := tuneInstrument(ctx, logger, name, manifest, workDir); err != nil {
			return err
		}
	}

	logger.InfoContext(ctx, circuit.LogTuneAllCompleted,
		slog.Any(circuit.LogKeyCount, len(names)))

	return nil
}

// tuneInstrument runs a single instrument's tune command.
func tuneInstrument(ctx context.Context, logger *slog.Logger, name string, manifest *circuit.InstrumentManifest, workDir string) error {
	logger.DebugContext(ctx, circuit.LogTuneStarted,
		slog.Any(circuit.LogKeyInstrument, name),
		slog.Any(circuit.LogKeyCommand, manifest.Tune))

	//nolint:gosec // command comes from validated instrument manifest, not user input
	cmd := exec.CommandContext(ctx, "bash", "-c", manifest.Tune)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %s: %w", ErrTuneFailed, name, ctx.Err())
		}
		stderrMsg := stderr.String()
		if stderrMsg != "" {
			return fmt.Errorf("%w: %s: command %q failed: %w\nstderr: %s", ErrTuneFailed, name, manifest.Tune, err, stderrMsg)
		}
		return fmt.Errorf("%w: %s: command %q failed: %w", ErrTuneFailed, name, manifest.Tune, err)
	}

	logger.DebugContext(ctx, circuit.LogTuneCompleted,
		slog.Any(circuit.LogKeyInstrument, name))

	return nil
}

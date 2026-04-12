package engine

// Category: Execution — instrument preflight verification.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"

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

// tuneInstrument runs a single instrument's tune command and optionally
// verifies the checksum of its stdout output.
func tuneInstrument(ctx context.Context, logger *slog.Logger, name string, manifest *circuit.InstrumentManifest, workDir string) error {
	logger.DebugContext(ctx, circuit.LogTuneStarted,
		slog.Any(circuit.LogKeyInstrument, name),
		slog.Any(circuit.LogKeyCommand, manifest.Tune))

	//nolint:gosec // command comes from validated instrument manifest, not user input
	cmd := exec.CommandContext(ctx, "bash", "-c", manifest.Tune)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
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

	if manifest.Checksum != "" {
		if err := verifyChecksum(name, stdout.Bytes(), manifest.Checksum); err != nil {
			return err
		}
	}

	logger.DebugContext(ctx, circuit.LogTuneCompleted,
		slog.Any(circuit.LogKeyInstrument, name))

	return nil
}

// ComputeChecksum runs an instrument's tune command and returns the
// "sha256:<hex>" string suitable for the manifest's checksum field.
// This is the generator half of the pin/verify cycle.
func ComputeChecksum(ctx context.Context, manifest *circuit.InstrumentManifest, workDir string) (string, error) {
	//nolint:gosec // command comes from validated instrument manifest, not user input
	cmd := exec.CommandContext(ctx, "bash", "-c", manifest.Tune)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tune command failed: %w", err)
	}
	hash := sha256.Sum256(stdout.Bytes())
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

// verifyChecksum compares sha256(stdout) against a declared "sha256:<hex>" checksum.
func verifyChecksum(name string, stdout []byte, declared string) error {
	prefix, want, ok := strings.Cut(declared, ":")
	if !ok || prefix != "sha256" {
		return fmt.Errorf("%w: %s: checksum must use format sha256:<hex>, got %q", ErrChecksumMismatch, name, declared)
	}

	hash := sha256.Sum256(stdout)
	got := hex.EncodeToString(hash[:])

	if got != want {
		return fmt.Errorf("%w: %s: expected sha256:%s, got sha256:%s", ErrChecksumMismatch, name, want, got)
	}
	return nil
}

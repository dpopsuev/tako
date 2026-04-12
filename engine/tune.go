package engine

// Category: Execution — instrument preflight verification.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
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

// tuneInstrument runs a single instrument's tune command (binary + tune args)
// and optionally verifies the binary file's checksum.
func tuneInstrument(ctx context.Context, logger *slog.Logger, name string, manifest *circuit.InstrumentManifest, workDir string) error {
	tuneCmd := manifest.Binary + " " + manifest.Tune

	logger.DebugContext(ctx, circuit.LogTuneStarted,
		slog.Any(circuit.LogKeyInstrument, name),
		slog.Any(circuit.LogKeyCommand, tuneCmd))

	//nolint:gosec // command comes from validated instrument manifest, not user input
	cmd := exec.CommandContext(ctx, "bash", "-c", tuneCmd)
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
			return fmt.Errorf("%w: %s: command %q failed: %w\nstderr: %s", ErrTuneFailed, name, tuneCmd, err, stderrMsg)
		}
		return fmt.Errorf("%w: %s: command %q failed: %w", ErrTuneFailed, name, tuneCmd, err)
	}

	if manifest.Checksum != "" {
		if err := verifyBinaryChecksum(name, manifest.Binary, manifest.Checksum); err != nil {
			return err
		}
	}

	logger.DebugContext(ctx, circuit.LogTuneCompleted,
		slog.Any(circuit.LogKeyInstrument, name))

	return nil
}

// ComputeChecksum resolves the binary on PATH and returns its file hash
// as "sha256:<hex>". This is the generator half of the pin/verify cycle.
func ComputeChecksum(manifest *circuit.InstrumentManifest) (string, error) {
	return hashBinary(manifest.Binary)
}

// verifyBinaryChecksum resolves the binary on PATH, hashes the file,
// and compares against the declared checksum.
func verifyBinaryChecksum(name, binary, declared string) error {
	got, err := hashBinary(binary)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrChecksumMismatch, name, err)
	}

	prefix, want, ok := strings.Cut(declared, ":")
	if !ok || prefix != "sha256" {
		return fmt.Errorf("%w: %s: checksum must use format sha256:<hex>, got %q", ErrChecksumMismatch, name, declared)
	}

	if got != "sha256:"+want {
		return fmt.Errorf("%w: %s: expected %s, got %s", ErrChecksumMismatch, name, declared, got)
	}
	return nil
}

// hashBinary resolves a binary name via PATH and returns sha256:<hex> of the file.
func hashBinary(binary string) (string, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("binary %q not found on PATH: %w", binary, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read binary %q: %w", path, err)
	}
	hash := sha256.Sum256(data)
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

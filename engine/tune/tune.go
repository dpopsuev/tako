// Package tune provides instrument preflight verification.
// TuneAll validates that every registered instrument's binary is available
// and responds to its tune command before the circuit walks.
package tune

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

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/circuit/def"
)

// Registry maps instrument names to their loaded manifests.
type Registry map[string]*def.InstrumentManifest

// All runs the preflight tune command for every instrument in the registry.
// Fails fast on the first failure. Instruments are tuned in sorted order
// for deterministic output.
func All(ctx context.Context, instruments Registry, workDir string) error {
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
		if err := instrument(ctx, logger, name, manifest, workDir); err != nil {
			return err
		}
	}

	logger.InfoContext(ctx, circuit.LogTuneAllCompleted,
		slog.Any(circuit.LogKeyCount, len(names)))

	return nil
}

// Sentinel errors.
var (
	ErrTuneFailed       = fmt.Errorf("tune failed")
	ErrChecksumMismatch = fmt.Errorf("checksum mismatch")
)

// instrument runs a single instrument's tune command and optionally verifies checksum.
func instrument(ctx context.Context, logger *slog.Logger, name string, manifest *def.InstrumentManifest, workDir string) error {
	tuneCmd := manifest.Binary + " " + manifest.Tune

	logger.DebugContext(ctx, circuit.LogTuneStarted,
		slog.Any(circuit.LogKeyInstrument, name),
		slog.Any(circuit.LogKeyCommand, tuneCmd))

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
		if err := VerifyBinaryChecksum(name, manifest.Binary, manifest.Checksum); err != nil {
			return err
		}
	}

	logger.DebugContext(ctx, circuit.LogTuneCompleted,
		slog.Any(circuit.LogKeyInstrument, name))

	return nil
}

// ComputeChecksum resolves the binary on PATH and returns its file hash as "sha256:<hex>".
func ComputeChecksum(manifest *def.InstrumentManifest) (string, error) {
	return hashBinary(manifest.Binary)
}

// VerifyBinaryChecksum resolves the binary on PATH, hashes the file,
// and compares against the declared checksum.
func VerifyBinaryChecksum(name, binary, declared string) error {
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

func hashBinary(binary string) (string, error) {
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("binary %q not found on PATH: %w", binary, err)
	}
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return "", fmt.Errorf("read binary %q: %w", binaryPath, err)
	}
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

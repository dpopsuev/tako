package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

const statusProcessing = "processing"

// FileDispatcherConfig configures the FileDispatcher behavior.
type FileDispatcherConfig struct {
	PollInterval    time.Duration // how often to check for the artifact; default 500ms
	Timeout         time.Duration // max time to wait for the artifact; default 10min
	MaxStaleRejects int           // consecutive stale dispatch_id reads before aborting; default 3
	SignalDir       string        // directory for signal.json; defaults to artifact dir
	Logger          *slog.Logger  // structured logger; nil = discard
}

// DefaultFileDispatcherConfig returns sensible defaults.
func DefaultFileDispatcherConfig() FileDispatcherConfig {
	return FileDispatcherConfig{
		PollInterval:    500 * time.Millisecond,
		Timeout:         10 * time.Minute,
		MaxStaleRejects: 10,
	}
}

// SignalFile is the JSON written next to the prompt to inform the external
// agent that a prompt is waiting.
type SignalFile struct {
	Status       string `json:"status"`      // waiting, processing, done, error
	DispatchID   int64  `json:"dispatch_id"` // monotonic ID; agent must echo in artifact wrapper
	CaseID       string `json:"case_id"`
	Step         string `json:"step"`
	PromptPath   string `json:"prompt_path"`
	ArtifactPath string `json:"artifact_path"`
	Timestamp    string `json:"timestamp"`
	Error        string `json:"error,omitempty"`
}

// ArtifactWrapper is a thin envelope the responder writes. The dispatcher
// accepts the artifact only when dispatch_id matches the current signal.
type ArtifactWrapper struct {
	DispatchID int64           `json:"dispatch_id"`
	Data       json.RawMessage `json:"data"`
}

// FileDispatcher writes a signal.json file and polls for the artifact file
// to appear on disk. Designed for automated/semi-automated calibration where
// an external agent watches for the signal.
type FileDispatcher struct {
	cfg        FileDispatcherConfig
	log        *slog.Logger
	dispatchID int64 // monotonic counter
}

// NewFileDispatcher creates a file-based dispatcher with the given config.
func NewFileDispatcher(cfg FileDispatcherConfig) *FileDispatcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	if cfg.MaxStaleRejects <= 0 {
		cfg.MaxStaleRejects = 10
	}
	l := cfg.Logger
	if l == nil {
		l = slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentFileDispatch))
	}
	return &FileDispatcher{cfg: cfg, log: l}
}

// Dispatch writes signal.json with a monotonic dispatch_id, polls for an
// artifact whose wrapper echoes the same dispatch_id, validates JSON, and
// returns the inner "data" bytes.
//
//nolint:funlen // sequential file-based dispatch protocol with polling loop
func (d *FileDispatcher) Dispatch(_ context.Context, ctx agentport.Context) ([]byte, error) {
	signalDir := d.cfg.SignalDir
	if signalDir == "" {
		signalDir = filepath.Dir(ctx.ArtifactPath)
	}
	signalPath := filepath.Join(signalDir, "signal.json")

	d.dispatchID++
	did := d.dispatchID

	dl := d.log.With(slog.Any(circuit.LogKeyCaseID, ctx.CaseID), slog.Any(circuit.LogKeyStep, ctx.Step), slog.Any(circuit.LogKeyDispatchID, did))
	dl.DebugContext(context.Background(), circuit.LogDispatchBegin, slog.Any(circuit.LogKeyArtifactPath, ctx.ArtifactPath), slog.Any(circuit.LogKeySignalPath, signalPath))

	// Remove any existing artifact file before writing the signal.
	if _, err := os.Stat(ctx.ArtifactPath); err == nil {
		dl.DebugContext(context.Background(), circuit.LogRemoveStaleArtifact, slog.Any(circuit.LogKeyPath, ctx.ArtifactPath))
		_ = os.Remove(ctx.ArtifactPath)
	}

	// Write signal: status=waiting with the new dispatch_id.
	sig := SignalFile{
		Status:       "waiting",
		DispatchID:   did,
		CaseID:       ctx.CaseID,
		Step:         ctx.Step,
		PromptPath:   ctx.PromptPath,
		ArtifactPath: ctx.ArtifactPath,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := WriteSignal(signalPath, &sig); err != nil {
		return nil, fmt.Errorf("write signal: %w", err)
	}
	dl.DebugContext(context.Background(), circuit.LogSignalWritten, slog.Any(circuit.LogKeyStatus, "waiting"))

	dl.InfoContext(context.Background(), circuit.LogSignalWaiting, slog.Any(circuit.LogKeyArtifactPath, ctx.ArtifactPath), slog.Any(circuit.LogKeyTimeout, d.cfg.Timeout))

	// Poll for artifact file with matching dispatch_id
	deadline := time.Now().Add(d.cfg.Timeout)
	pollCount := 0
	staleCount := 0
	for {
		if time.Now().After(deadline) {
			dl.DebugContext(context.Background(), circuit.LogTimeoutReached, slog.Any(circuit.LogKeyPolls, pollCount))
			sig.Status = statusError
			sig.Error = "timeout waiting for artifact"
			_ = WriteSignal(signalPath, &sig)
			return nil, fmt.Errorf("%w: %s waiting for artifact at %s", ErrTimeoutAfter, d.cfg.Timeout, ctx.ArtifactPath)
		}

		// Check if the responder reported an error via signal.json
		if sigData, readErr := os.ReadFile(signalPath); readErr == nil {
			var liveSig SignalFile
			if json.Unmarshal(sigData, &liveSig) == nil && liveSig.DispatchID == did && liveSig.Status == statusError {
				dl.DebugContext(context.Background(), circuit.LogResponderError, slog.Any(circuit.LogKeyError, liveSig.Error))
				return nil, fmt.Errorf("%w: %s", ErrResponderError, liveSig.Error)
			}
		}

		pollCount++
		data, err := os.ReadFile(ctx.ArtifactPath)
		if err != nil {
			if pollCount <= 3 || pollCount%20 == 0 {
				dl.DebugContext(context.Background(), circuit.LogPollArtifactNotFound, slog.Any(circuit.LogKeyPoll, pollCount), slog.Any(circuit.LogKeyError, err))
			}
			staleCount = 0
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		dl.DebugContext(context.Background(), circuit.LogPollArtifactFound, slog.Any(circuit.LogKeyPoll, pollCount), slog.Any(circuit.LogKeyBytes, len(data)))

		// Parse the wrapper to check dispatch_id.
		// Invalid JSON is treated as transient (partial write from a concurrent
		// writer) and counts toward the stale tolerance, same as a dispatch_id
		// mismatch. This avoids hard-failing on a race between os.WriteFile in
		// the responder and os.ReadFile here.
		var wrapper ArtifactWrapper
		if err := json.Unmarshal(data, &wrapper); err != nil {
			staleCount++
			dl.DebugContext(context.Background(), circuit.LogPollInvalidJSON, slog.Any(circuit.LogKeyPoll, pollCount), slog.Any(circuit.LogKeyError, err), slog.Any(circuit.LogKeyBadReadStreak, staleCount), slog.Any(circuit.LogKeyMax, d.cfg.MaxStaleRejects))
			if staleCount >= d.cfg.MaxStaleRejects {
				sig.Status = statusError
				sig.Error = fmt.Sprintf("stale artifact tolerance exceeded: %d consecutive unusable reads (last: invalid JSON: %v)",
					staleCount, err)
				_ = WriteSignal(signalPath, &sig)
				return nil, fmt.Errorf("stale artifact tolerance exceeded: %d consecutive unusable reads at %s (last: invalid JSON: %w)",
					staleCount, ctx.ArtifactPath, err)
			}
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		// Reject stale artifacts deterministically by dispatch_id
		if wrapper.DispatchID != did {
			staleCount++
			dl.DebugContext(context.Background(), circuit.LogPollStaleArtifact, slog.Any(circuit.LogKeyPoll, pollCount), slog.Any(circuit.LogKeyWant, did), slog.Any(circuit.LogKeyGot, wrapper.DispatchID), slog.Any(circuit.LogKeyStaleStreak, staleCount), slog.Any(circuit.LogKeyMax, d.cfg.MaxStaleRejects))
			if staleCount >= d.cfg.MaxStaleRejects {
				sig.Status = statusError
				sig.Error = fmt.Sprintf("exceeded stale tolerance: %d consecutive artifacts with wrong dispatch_id (want %d, last got %d)",
					staleCount, did, wrapper.DispatchID)
				_ = WriteSignal(signalPath, &sig)
				return nil, fmt.Errorf("%w: %d consecutive dispatch_id mismatches (want %d, got %d) at %s", ErrStaleArtifactToleranceExceeded, staleCount, did, wrapper.DispatchID, ctx.ArtifactPath)
			}
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		// dispatch_id matches — this is our artifact
		if len(wrapper.Data) == 0 {
			sig.Status = statusError
			sig.Error = "artifact wrapper has empty 'data' field"
			_ = WriteSignal(signalPath, &sig)
			return nil, fmt.Errorf("%w: %s has matching dispatch_id but empty 'data'", ErrArtifactAt, ctx.ArtifactPath)
		}

		dl.DebugContext(context.Background(), circuit.LogArtifactValidated, slog.Any(circuit.LogKeyPoll, pollCount), slog.Any(circuit.LogKeyBytes, len(wrapper.Data)))

		// Update signal: status=processing
		sig.Status = statusProcessing
		sig.Error = ""
		_ = WriteSignal(signalPath, &sig)

		dl.InfoContext(context.Background(), circuit.LogArtifactAccepted, slog.Any(circuit.LogKeyBytes, len(wrapper.Data)))
		return wrapper.Data, nil
	}
}

// MarkDone updates the signal file to "done" after the caller has processed the artifact.
func (d *FileDispatcher) MarkDone(artifactPath string) {
	signalDir := d.cfg.SignalDir
	if signalDir == "" {
		signalDir = filepath.Dir(artifactPath)
	}
	signalPath := filepath.Join(signalDir, "signal.json")

	data, err := os.ReadFile(signalPath)
	if err != nil {
		d.log.DebugContext(context.Background(), circuit.LogMarkDoneReadFailed, slog.Any(circuit.LogKeyPath, signalPath), slog.Any(circuit.LogKeyError, err))
		return
	}
	var sig SignalFile
	if err := json.Unmarshal(data, &sig); err != nil {
		d.log.DebugContext(context.Background(), circuit.LogMarkDoneParseFailed, slog.Any(circuit.LogKeyPath, signalPath), slog.Any(circuit.LogKeyError, err))
		return
	}
	d.log.DebugContext(context.Background(), circuit.LogMarkDone, slog.Any(circuit.LogKeyStatus, sig.Status), slog.Any(circuit.LogKeyCaseID, sig.CaseID), slog.Any(circuit.LogKeyStep, sig.Step), slog.Any(circuit.LogKeyDispatchID, sig.DispatchID))
	sig.Status = statusDone
	_ = WriteSignal(signalPath, &sig)
}

// CurrentDispatchID returns the latest dispatch_id. Useful for tests.
func (d *FileDispatcher) CurrentDispatchID() int64 {
	return d.dispatchID
}

// WriteSignal atomically writes a signal file.
func WriteSignal(path string, sig *SignalFile) error {
	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write signal tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		defer os.Remove(tmp)
		return os.WriteFile(path, data, 0o600)
	}
	return nil
}

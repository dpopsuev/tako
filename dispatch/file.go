package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	bd "github.com/dpopsuev/bugle/dispatch"
)

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
	Status       string `json:"status"`        // waiting, processing, done, error
	DispatchID   int64  `json:"dispatch_id"`   // monotonic ID; agent must echo in artifact wrapper
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
		l = slog.Default().With("component", "file-dispatch")
	}
	return &FileDispatcher{cfg: cfg, log: l}
}

// Dispatch writes signal.json with a monotonic dispatch_id, polls for an
// artifact whose wrapper echoes the same dispatch_id, validates JSON, and
// returns the inner "data" bytes.
func (d *FileDispatcher) Dispatch(_ context.Context, ctx bd.Context) ([]byte, error) {
	signalDir := d.cfg.SignalDir
	if signalDir == "" {
		signalDir = filepath.Dir(ctx.ArtifactPath)
	}
	signalPath := filepath.Join(signalDir, "signal.json")

	d.dispatchID++
	did := d.dispatchID

	dl := d.log.With("case", ctx.CaseID, "step", ctx.Step, "dispatch_id", did)
	dl.Debug("dispatch begin", "artifact_path", ctx.ArtifactPath, "signal_path", signalPath)

	// Remove any existing artifact file before writing the signal.
	if _, err := os.Stat(ctx.ArtifactPath); err == nil {
		dl.Debug("removing stale artifact before dispatch", "path", ctx.ArtifactPath)
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
	dl.Debug("signal written", "status", "waiting")

	dl.Info("signal.json written, waiting for artifact",
		"artifact_path", ctx.ArtifactPath, "timeout", d.cfg.Timeout)

	// Poll for artifact file with matching dispatch_id
	deadline := time.Now().Add(d.cfg.Timeout)
	pollCount := 0
	staleCount := 0
	for {
		if time.Now().After(deadline) {
			dl.Debug("timeout reached", "polls", pollCount)
			sig.Status = "error"
			sig.Error = "timeout waiting for artifact"
			_ = WriteSignal(signalPath, &sig)
			return nil, fmt.Errorf("timeout after %s waiting for artifact at %s", d.cfg.Timeout, ctx.ArtifactPath)
		}

		// Check if the responder reported an error via signal.json
		if sigData, readErr := os.ReadFile(signalPath); readErr == nil {
			var liveSig SignalFile
			if json.Unmarshal(sigData, &liveSig) == nil && liveSig.DispatchID == did && liveSig.Status == "error" {
				dl.Debug("responder reported error via signal", "error", liveSig.Error)
				return nil, fmt.Errorf("responder error: %s", liveSig.Error)
			}
		}

		pollCount++
		data, err := os.ReadFile(ctx.ArtifactPath)
		if err != nil {
			if pollCount <= 3 || pollCount%20 == 0 {
				dl.Debug("poll: artifact not found", "poll", pollCount, "err", err)
			}
			staleCount = 0
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		dl.Debug("poll: artifact file found", "poll", pollCount, "bytes", len(data))

		// Parse the wrapper to check dispatch_id.
		// Invalid JSON is treated as transient (partial write from a concurrent
		// writer) and counts toward the stale tolerance, same as a dispatch_id
		// mismatch. This avoids hard-failing on a race between os.WriteFile in
		// the responder and os.ReadFile here.
		var wrapper ArtifactWrapper
		if err := json.Unmarshal(data, &wrapper); err != nil {
			staleCount++
			dl.Debug("poll: invalid JSON (possible partial write)",
				"poll", pollCount, "err", err,
				"bad_read_streak", staleCount, "max", d.cfg.MaxStaleRejects)
			if staleCount >= d.cfg.MaxStaleRejects {
				sig.Status = "error"
				sig.Error = fmt.Sprintf("stale artifact tolerance exceeded: %d consecutive unusable reads (last: invalid JSON: %v)",
					staleCount, err)
				_ = WriteSignal(signalPath, &sig)
				return nil, fmt.Errorf("stale artifact tolerance exceeded: %d consecutive unusable reads at %s (last: invalid JSON: %v)",
					staleCount, ctx.ArtifactPath, err)
			}
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		// Reject stale artifacts deterministically by dispatch_id
		if wrapper.DispatchID != did {
			staleCount++
			dl.Debug("poll: stale artifact (dispatch_id mismatch)",
				"poll", pollCount, "want", did, "got", wrapper.DispatchID,
				"stale_streak", staleCount, "max", d.cfg.MaxStaleRejects)
			if staleCount >= d.cfg.MaxStaleRejects {
				sig.Status = "error"
				sig.Error = fmt.Sprintf("exceeded stale tolerance: %d consecutive artifacts with wrong dispatch_id (want %d, last got %d)",
					staleCount, did, wrapper.DispatchID)
				_ = WriteSignal(signalPath, &sig)
				return nil, fmt.Errorf("stale artifact tolerance exceeded: %d consecutive dispatch_id mismatches (want %d, got %d) at %s",
					staleCount, did, wrapper.DispatchID, ctx.ArtifactPath)
			}
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		// dispatch_id matches — this is our artifact
		if len(wrapper.Data) == 0 {
			sig.Status = "error"
			sig.Error = "artifact wrapper has empty 'data' field"
			_ = WriteSignal(signalPath, &sig)
			return nil, fmt.Errorf("artifact at %s has matching dispatch_id but empty 'data'", ctx.ArtifactPath)
		}

		dl.Debug("artifact validated", "poll", pollCount, "bytes", len(wrapper.Data))

		// Update signal: status=processing
		sig.Status = "processing"
		sig.Error = ""
		_ = WriteSignal(signalPath, &sig)

		dl.Info("artifact validated and accepted", "bytes", len(wrapper.Data))
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
		d.log.Debug("mark-done: cannot read signal", "path", signalPath, "err", err)
		return
	}
	var sig SignalFile
	if err := json.Unmarshal(data, &sig); err != nil {
		d.log.Debug("mark-done: cannot parse signal", "path", signalPath, "err", err)
		return
	}
	d.log.Debug("mark-done", "prev_status", sig.Status, "case", sig.CaseID, "step", sig.Step, "dispatch_id", sig.DispatchID)
	sig.Status = "done"
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
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write signal tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		defer os.Remove(tmp)
		return os.WriteFile(path, data, 0644)
	}
	return nil
}

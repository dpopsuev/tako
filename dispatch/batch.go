package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

const (
	statusError = "error"
	statusDone  = "done"
)

// BatchFileDispatcher writes N signals concurrently, generates a batch
// manifest and briefing file, then polls all N artifact paths in parallel.
type BatchFileDispatcher struct {
	cfg         FileDispatcherConfig
	log         *slog.Logger
	suiteDir    string
	batchID     int64
	batchSize   int
	tokenBudget int
	tokenUsed   int
}

// BatchFileDispatcherConfig configures the BatchFileDispatcher.
type BatchFileDispatcherConfig struct {
	FileConfig  FileDispatcherConfig
	SuiteDir    string
	BatchSize   int
	TokenBudget int
	Logger      *slog.Logger
}

// NewBatchFileDispatcher creates a batch dispatcher.
func NewBatchFileDispatcher(cfg *BatchFileDispatcherConfig) *BatchFileDispatcher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 4
	}
	l := cfg.Logger
	if l == nil {
		l = slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentBatchDispatch))
	}
	return &BatchFileDispatcher{
		cfg:         cfg.FileConfig,
		log:         l,
		suiteDir:    cfg.SuiteDir,
		batchSize:   cfg.BatchSize,
		tokenBudget: cfg.TokenBudget,
	}
}

// BatchResult holds the result of dispatching one signal in a batch.
type BatchResult struct {
	Index int
	Data  []byte
	Err   error
}

// DispatchBatch writes N signals, generates a manifest and briefing path,
// then polls all N artifact paths concurrently.
func (d *BatchFileDispatcher) DispatchBatch(ctx context.Context, ctxs []agentport.Context, phase, briefingPath string) ([][]byte, []error) {
	n := len(ctxs)
	if n == 0 {
		return nil, nil
	}

	d.batchID++
	bid := d.batchID

	d.log.DebugContext(ctx, circuit.LogBatchDispatchBegin, slog.Any(circuit.LogKeyBatchID, bid), slog.Any(circuit.LogKeySignals, n), slog.Any(circuit.LogKeyPhase, phase))

	signals := make([]BatchSignalEntry, n)
	for i, dc := range ctxs {
		sigDir := filepath.Dir(dc.ArtifactPath)
		signals[i] = BatchSignalEntry{
			CaseID:     dc.CaseID,
			SignalPath: filepath.Join(sigDir, "signal.json"),
			Status:     "pending",
		}
	}

	manifest := NewBatchManifest(bid, phase, briefingPath, signals)
	manifestPath := filepath.Join(d.suiteDir, "batch-manifest.json")
	if err := os.MkdirAll(d.suiteDir, 0o755); err != nil {
		errs := make([]error, n)
		for i := range errs {
			errs[i] = fmt.Errorf("mkdir suite dir: %w", err)
		}
		return make([][]byte, n), errs
	}
	if err := WriteManifest(manifestPath, manifest); err != nil {
		errs := make([]error, n)
		for i := range errs {
			errs[i] = fmt.Errorf("write manifest: %w", err)
		}
		return make([][]byte, n), errs
	}

	d.log.DebugContext(ctx, circuit.LogManifestWritten, slog.Any(circuit.LogKeyBatchID, bid), slog.Any(circuit.LogKeyPath, manifestPath), slog.Any(circuit.LogKeySignals, n))

	manifest.Status = "in_progress"
	_ = WriteManifest(manifestPath, manifest)

	results := make([]BatchResult, n)
	var wg sync.WaitGroup

	for i, dc := range ctxs {
		wg.Add(1)
		go func(idx int, dctx agentport.Context) {
			defer wg.Done()
			fd := NewFileDispatcher(d.cfg)
			data, err := fd.Dispatch(ctx, dctx)
			results[idx] = BatchResult{
				Index: idx,
				Data:  data,
				Err:   err,
			}
			if err != nil {
				signals[idx].Status = statusError
			} else {
				signals[idx].Status = statusDone
			}
		}(i, dc)
	}

	wg.Wait()

	allError := true
	for _, r := range results {
		if r.Err == nil {
			allError = false
		}
	}

	switch {
	case allError:
		manifest.Status = statusError
	default:
		manifest.Status = statusDone
	}
	manifest.Signals = signals
	_ = WriteManifest(manifestPath, manifest)

	if d.tokenBudget > 0 {
		budgetPath := filepath.Join(d.suiteDir, "budget-status.json")
		if err := WriteBudgetStatus(budgetPath, d.tokenBudget, d.tokenUsed); err != nil {
			d.log.WarnContext(ctx, circuit.LogBudgetStatusFailed, slog.Any(circuit.LogKeyError, err))
		}
	}

	d.log.DebugContext(ctx, circuit.LogBatchDispatchComplete, slog.Any(circuit.LogKeyBatchID, bid), slog.Any(circuit.LogKeyStatus, manifest.Status))

	data := make([][]byte, n)
	errs := make([]error, n)
	for _, r := range results {
		data[r.Index] = r.Data
		errs[r.Index] = r.Err
	}

	return data, errs
}

// Dispatch implements the Dispatcher interface for single-signal compatibility.
func (d *BatchFileDispatcher) Dispatch(ctx context.Context, dc agentport.Context) ([]byte, error) {
	data, errs := d.DispatchBatch(ctx, []agentport.Context{dc}, "single", "")
	if len(errs) > 0 && errs[0] != nil {
		return nil, errs[0]
	}
	if len(data) > 0 {
		return data[0], nil
	}
	return nil, ErrBatchDispatchReturnedNoResults
}

// SuiteDir returns the configured suite directory.
func (d *BatchFileDispatcher) SuiteDir() string {
	return d.suiteDir
}

// BatchSize returns the configured maximum batch size.
func (d *BatchFileDispatcher) BatchSize() int {
	return d.batchSize
}

// ManifestPath returns the path to the batch manifest for the current suite.
func (d *BatchFileDispatcher) ManifestPath() string {
	return filepath.Join(d.suiteDir, "batch-manifest.json")
}

// WriteBriefing writes a briefing file to the suite directory and returns its path.
func (d *BatchFileDispatcher) WriteBriefing(content string) (string, error) {
	path := filepath.Join(d.suiteDir, "briefing.md")
	if err := os.MkdirAll(d.suiteDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir for briefing: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write briefing: %w", err)
	}
	return path, nil
}

// LastBatchID returns the latest batch ID.
func (d *BatchFileDispatcher) LastBatchID() int64 {
	return d.batchID
}

// UpdateTokenUsage sets the cumulative token usage.
func (d *BatchFileDispatcher) UpdateTokenUsage(used int) {
	d.tokenUsed = used
}

// TokenBudget returns the configured token budget.
func (d *BatchFileDispatcher) TokenBudget() int {
	return d.tokenBudget
}

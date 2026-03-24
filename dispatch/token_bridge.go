package dispatch

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dpopsuev/bugle/billing"
	"github.com/dpopsuev/origami/format"
)

// DispatchHook is called after each dispatch with timing and error info.
type DispatchHook func(provider, step string, duration time.Duration, err error)

// TokenTrackingDispatcher wraps any Dispatcher and records token usage
// for each dispatch call. Optional DispatchHooks receive timing/error data
// for bridging with metrics systems.
type TokenTrackingDispatcher struct {
	inner         Dispatcher
	tracker       billing.Tracker
	provider      string
	dispatchHooks []DispatchHook
}

// NewTokenTrackingDispatcher wraps a dispatcher with token tracking.
func NewTokenTrackingDispatcher(inner Dispatcher, tracker billing.Tracker) *TokenTrackingDispatcher {
	return &TokenTrackingDispatcher{inner: inner, tracker: tracker}
}

// SetProvider sets the provider label used for dispatch hooks.
func (d *TokenTrackingDispatcher) SetProvider(name string) {
	d.provider = name
}

// OnDispatch registers a hook invoked after each Dispatch call.
func (d *TokenTrackingDispatcher) OnDispatch(hook DispatchHook) {
	d.dispatchHooks = append(d.dispatchHooks, hook)
}

// Dispatch delegates to the inner dispatcher while recording token metrics.
func (d *TokenTrackingDispatcher) Dispatch(ctx context.Context, dc DispatchContext) ([]byte, error) {
	promptBytes := 0
	if info, err := os.Stat(dc.PromptPath); err == nil {
		promptBytes = int(info.Size())
	}

	provider := d.provider
	if dc.Provider != "" {
		provider = dc.Provider
	}

	start := time.Now()
	data, err := d.inner.Dispatch(ctx, dc)
	elapsed := time.Since(start)

	for _, h := range d.dispatchHooks {
		h(provider, dc.Step, elapsed, err)
	}

	if err != nil {
		return data, err
	}

	artifactBytes := len(data)

	d.tracker.Record(&billing.TokenRecord{
		CaseID:         dc.CaseID,
		Step:           dc.Step,
		PromptBytes:    promptBytes,
		ArtifactBytes:  artifactBytes,
		PromptTokens:   billing.EstimateTokens(promptBytes),
		ArtifactTokens: billing.EstimateTokens(artifactBytes),
		Timestamp:      start,
		WallClockMs:    elapsed.Milliseconds(),
	})

	return data, nil
}

// Inner returns the wrapped dispatcher for type-specific operations.
func (d *TokenTrackingDispatcher) Inner() Dispatcher {
	return d.inner
}

// FormatTokenSummary returns a human-readable token and cost section.
// An optional CostConfig overrides the default pricing for per-line cost
// breakdown. If omitted, DefaultCostConfig() is used.
func FormatTokenSummary(s billing.TokenSummary, opts ...billing.CostConfig) string {
	cc := billing.DefaultCostConfig()
	if len(opts) > 0 {
		cc = opts[0]
	}

	avgPerCase := 0
	if len(s.PerCase) > 0 {
		avgPerCase = s.TotalTokens / len(s.PerCase)
	}
	avgPerStep := 0
	if s.TotalSteps > 0 {
		avgPerStep = s.TotalTokens / s.TotalSteps
	}

	wallSec := float64(s.TotalWallClockMs) / 1000.0
	minutes := int(wallSec) / 60
	seconds := int(wallSec) % 60

	promptCost := float64(s.TotalPromptTokens) / 1_000_000 * cc.InputPricePerMToken
	artifactCost := float64(s.TotalArtifactTokens) / 1_000_000 * cc.OutputPricePerMToken

	tbl := format.NewTable(format.ASCII)
	tbl.Header("Metric", "Value")
	tbl.Columns(
		format.ColumnConfig{Number: 1, Align: format.AlignLeft},
		format.ColumnConfig{Number: 2, Align: format.AlignRight},
	)
	tbl.Row("Total prompts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalPromptTokens, promptCost))
	tbl.Row("Total artifacts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalArtifactTokens, artifactCost))
	tbl.Row("Total", fmt.Sprintf("%d tokens ($%.4f)", s.TotalTokens, s.TotalCostUSD))
	tbl.Row("Per case avg", fmt.Sprintf("%d tokens", avgPerCase))
	tbl.Row("Per step avg", fmt.Sprintf("%d tokens", avgPerStep))
	tbl.Row("Steps", fmt.Sprintf("%d", s.TotalSteps))
	tbl.Row("Wall clock", fmt.Sprintf("%dm %ds", minutes, seconds))

	return "=== Token & Cost ===\n" + tbl.String() + "\n"
}

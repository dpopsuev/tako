package operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/origami/engine/trace"
)

// Log keys.
const (
	logKeyError      = "error"
	logKeyActResult  = "act_result"
	logKeyDurationMs = "duration_ms"
)

// ActStationLog records the outcome of an Actor.Act call for FlightRecorder.
type ActStationLog struct {
	Success    bool
	Error      string
	DurationMs int64
}

// StationLogType implements trace.StationLogger.
func (a *ActStationLog) StationLogType() string { return "act" }

// Config controls the reconciliation loop.
type Config struct {
	Desired  DesiredState
	Observer Observer
	Actor    Actor
	Interval time.Duration
	Recorder *trace.FlightRecorder
	MaxRuns  int // 0 = unlimited
}

// Loop runs the observe→diff→act reconciliation loop until the context
// is canceled or MaxRuns is reached. Returns the number of circuit runs
// executed.
func Loop(ctx context.Context, cfg Config) int {
	recorder := cfg.Recorder
	if recorder == nil {
		recorder = trace.NewFlightRecorder(1000)
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	runs := 0
	recorder.Record("operator:start", "in", "reconciliation loop started", nil, nil)

	for {
		if cfg.MaxRuns > 0 && runs >= cfg.MaxRuns {
			recorder.Record("operator:max-runs", "out", "max runs reached", nil, nil)
			break
		}

		select {
		case <-ctx.Done():
			recorder.Record("operator:shutdown", "out", "context canceled", nil, ctx.Err())
			return runs
		default:
		}

		// Observe.
		recorder.Record("operator:observe", "in", "snapshot current state", nil, nil)
		current, err := cfg.Observer.Observe()
		if err != nil {
			slog.ErrorContext(ctx, "operator observe failed", slog.Any(logKeyError, err))
			recorder.Record("operator:observe", "out", "error", nil, err)
			sleep(ctx, interval)
			continue
		}
		recorder.Record("operator:observe", "out", "state captured", &trace.TextLog{Text: "head=" + current.HeadSHA}, nil)

		// Diff.
		drift := Diff(cfg.Desired, current)
		recorder.Record("operator:diff", "out", driftSummary(drift), &trace.TextLog{Text: driftSummary(drift)}, nil)

		if !drift.Drifted {
			recorder.Record("operator:converged", "out", "no drift", nil, nil)
			sleep(ctx, interval)
			continue
		}

		// Act.
		recorder.Record("operator:act", "in", "running circuit", &trace.TextLog{Text: driftSummary(drift)}, nil)
		actStart := time.Now()
		result, err := cfg.Actor.Act(drift)
		actDuration := time.Since(actStart)
		runs++
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "operator act failed", slog.Any(logKeyError, err))
			recorder.Record("operator:act", "out", "error", &ActStationLog{
				Success:    false,
				Error:      err.Error(),
				DurationMs: actDuration.Milliseconds(),
			}, err)
		case result.Error != "":
			slog.ErrorContext(ctx, "operator act returned error",
				slog.String(logKeyActResult, result.Error),
				slog.Int64(logKeyDurationMs, actDuration.Milliseconds()))
			recorder.Record("operator:act", "out", "circuit error", &ActStationLog{
				Success:    result.Success,
				Error:      result.Error,
				DurationMs: actDuration.Milliseconds(),
			}, nil)
		default:
			recorder.Record("operator:act", "out", "circuit complete", &ActStationLog{
				Success:    result.Success,
				DurationMs: actDuration.Milliseconds(),
			}, nil)
		}

		sleep(ctx, interval)
	}

	recorder.Record("operator:stop", "out", "loop ended", &trace.TextLog{Text: fmt.Sprintf("runs=%d", runs)}, nil)
	return runs
}

// Diff compares desired state to current state and returns drift.
func Diff(desired DesiredState, current *CurrentState) DriftResult {
	var reasons []string

	if desired.Scan == "clean" && current.ScanFindings > 0 {
		reasons = append(reasons, "scan: findings detected")
	}
	if desired.Build == "passing" && !current.BuildPassing {
		reasons = append(reasons, "build: not passing")
	}
	if desired.Test == "passing" && !current.TestPassing {
		reasons = append(reasons, "test: not passing")
	}
	if current.Vulnerabilities > desired.Vulnerabilities {
		reasons = append(reasons, "vulnerabilities: above threshold")
	}

	return DriftResult{
		Drifted: len(reasons) > 0,
		Reasons: reasons,
	}
}

func driftSummary(d DriftResult) string {
	if !d.Drifted {
		return "converged"
	}
	return d.Reasons[0]
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

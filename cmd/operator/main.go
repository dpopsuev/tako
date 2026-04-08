// Command operator runs the SDLC reconciliation loop. It watches a
// repository for drift from a desired state and spawns circuit runs
// to converge.
//
// Usage:
//
//	operator --repo /path/to/origami --mode in-process
//	operator --repo /path/to/origami --mode container --image origami-sdlc:latest
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

const (
	logKeyRepo      = "repo"
	logKeyMode      = "mode"
	logKeyInterval  = "interval"
	logKeyTotalRuns = "total_runs"
)

func main() {
	repoPath := flag.String("repo", ".", "repository path to watch")
	mode := flag.String("mode", "in-process", "actor mode: in-process or container")
	image := flag.String("image", "origami-sdlc:latest", "container image for circuit runner")
	runtime := flag.String("runtime", "docker", "container runtime: docker or podman")
	interval := flag.Duration("interval", 30*time.Second, "poll interval")
	maxRuns := flag.Int("max-runs", 0, "max circuit runs (0 = unlimited)")
	flag.Parse()

	desired := operator.DesiredState{
		Manifest: "origami-sdlc.yaml",
		RepoPath: *repoPath,
		Scan:     "clean",
		Build:    "passing",
		Test:     "passing",
	}

	observer := operator.NewGitObserver(*repoPath)

	var actor operator.Actor
	switch *mode {
	case "in-process":
		// Set env so the factory picks up repo path and mode.
		os.Setenv(sdlc.EnvRepoPath, *repoPath)
		os.Setenv(sdlc.EnvMode, "real")

		factory := sdlc.SessionFactory()
		actor = operator.NewInProcessActor(func(ctx context.Context, _ operator.DriftResult) (*operator.RunResult, error) {
			start := time.Now()
			cfg, err := factory.CreateSession(ctx, &engine.SessionParams{})
			if err != nil {
				return &operator.RunResult{Success: false, Duration: time.Since(start), Error: err.Error()}, nil
			}
			result, err := sdlc.Run(ctx, sdlc.RunConfig{
				Transformers: cfg.Transformers,
			})
			if err != nil {
				return &operator.RunResult{Success: false, Duration: time.Since(start), Error: err.Error()}, nil
			}
			for _, wr := range result.WalkResults {
				if wr.Error != nil {
					return &operator.RunResult{Success: false, Duration: time.Since(start), Error: wr.Error.Error()}, nil
				}
			}
			return &operator.RunResult{Success: true, Duration: time.Since(start)}, nil
		})
	case "container":
		actor = operator.NewContainerActor(*image, *repoPath, operator.WithRuntime(*runtime))
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s (use in-process or container)\n", *mode)
		os.Exit(1)
	}

	recorder := trace.NewFlightRecorder(5000)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.InfoContext(ctx, "operator starting",
		slog.String(logKeyRepo, *repoPath),
		slog.String(logKeyMode, *mode),
		slog.Duration(logKeyInterval, *interval),
	)

	runs := operator.Loop(ctx, operator.Config{
		Desired:  desired,
		Observer: observer,
		Actor:    actor,
		Interval: *interval,
		Recorder: recorder,
		MaxRuns:  *maxRuns,
	})

	slog.InfoContext(ctx, "operator stopped", slog.Int(logKeyTotalRuns, runs))
}

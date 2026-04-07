package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/dispatch/guard"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
)

// Log event names specific to session bridge observability.
const (
	LogObservabilityComputed = "observability metrics computed"
)

// Dispatch mode constants for Extra["dispatch"].
const (
	DispatchModeMCP = "mcp" // default: external workers via step/submit protocol
	DispatchModeACP = "acp" // in-process ACP agent workers

	ExtraKeyDispatch        = "dispatch"
	ExtraKeyDispatchCommand = "dispatch_command"

	defaultACPAgent = "cursor"
	defaultACPRole  = "worker"
)

// SessionFactoryToConfig bridges engine.SessionFactory (interface API) to
// CircuitConfig.CreateSession (callback API). The framework creates
// dispatcher and bus internally, then builds a RunFunc from the
// consumer's SessionConfig.
//
// Optional capabilities (ReportFormatter, StepSchemaProvider) are detected
// via type assertion on the factory, following the DeterministicTransformer
// pattern in engine/transformer.go.
func SessionFactoryToConfig(factory engine.SessionFactory) CircuitConfig {
	cfg := CircuitConfig{
		CreateSession: func(ctx context.Context, params StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (RunFunc, SessionMeta, error) {
			// TSK-515: Auto-wire token tracking — consumers get it for free.
			tracker := agentport.NewTracker()
			trackedDisp := guard.NewTokenTrackingDispatcher(disp, tracker)

			engineParams := engine.SessionParams{
				Parallel:            params.Parallel,
				Force:               params.Force,
				Extra:               params.Extra,
				DomainFS:            params.DomainFS,
				StateDir:            params.StateDir,
				Observer:            params.Observer,
				Dispatcher:          trackedDisp,
				Relayer:             &dispatch.MuxRelayer{Disp: disp},
				PromptStore:         params.PromptStore,
				ResourceRegistry:    params.ResourceRegistry,
				SubCircuitResolvers: params.SubCircuitResolvers,
			}

			sessionCfg, err := factory.CreateSession(ctx, &engineParams)
			if err != nil {
				return nil, SessionMeta{}, err
			}

			// Auto-check component health before consumer preflight.
			if healthComps := collectHealthComponents(sessionCfg); len(healthComps) > 0 {
				if err := engine.CheckComponentHealth(ctx, healthComps); err != nil {
					return nil, SessionMeta{}, fmt.Errorf("health check: %w", err)
				}
			}

			// Run consumer preflight after health checks pass.
			if sessionCfg.Preflight != nil {
				if err := sessionCfg.Preflight(ctx); err != nil {
					return nil, SessionMeta{}, fmt.Errorf("preflight: %w", err)
				}
			}

			runFn := buildRunFunc(sessionCfg, &engineParams, params.SubCircuitResolvers, tracker)

			// ACP dispatch mode: spawn in-process ACP agent workers that
			// bridge MuxDispatcher <-> agent CLIs. No external workers needed.
			if dispatchMode, _ := params.Extra[ExtraKeyDispatch].(string); dispatchMode == DispatchModeACP {
				runFn = wrapWithACPWorkers(runFn, params, disp, bus)
			}

			_ = bus // framework-created, available via signal tools

			meta := SessionMeta{
				TotalCases: sessionCfg.Meta.TotalCases,
				Scenario:   sessionCfg.Meta.Scenario,
			}
			return runFn, meta, nil
		},
	}

	if rf, ok := factory.(engine.ReportFormatter); ok {
		cfg.FormatReport = rf.FormatReport
	}
	if sp, ok := factory.(engine.StepSchemaProvider); ok {
		cfg.StepSchemas = sp.StepSchemas()
	}

	return cfg
}

func buildRunFunc(cfg *engine.SessionConfig, params *engine.SessionParams, resolvers map[string]circuit.AssetResolver, tracker agentport.Tracker) RunFunc {
	if cfg.RunFunc != nil {
		return cfg.RunFunc
	}
	return func(ctx context.Context) (any, error) {
		shared := &engine.GraphRegistries{
			Transformers: cfg.Transformers,
			Extractors:   cfg.Extractors,
			Hooks:        cfg.Hooks,
		}
		// Load sub-circuit definitions from resolvers (e.g., GND within RCA).
		if len(resolvers) > 0 && params.DomainFS != nil {
			shared.Circuits = def.LoadSubCircuitsFromFS(params.DomainFS, resolvers)
		}

		// TSK-516: Auto-collect trace events for step latency computation.
		collector := &trace.TraceCollector{}
		var observer circuit.WalkObserver
		if params.Observer != nil {
			observer = circuit.MultiObserver{collector, params.Observer}
		} else {
			observer = collector
		}

		bwCfg := engine.BatchWalkConfig{
			Def:      cfg.CircuitDef,
			Shared:   shared,
			Cases:    cfg.Cases,
			Parallel: params.Parallel,
			Observer: observer,
		}
		results := engine.BatchWalk(ctx, bwCfg)

		// TSK-516: Compute per-node step latency from trace events.
		stepLatency := computeStepLatency(collector.Events())

		// TSK-517: Compute per-node error rates from batch results.
		errorRates := computeErrorRates(results)

		// TSK-515: Include token summary in the session report.
		var tokenSummary *agentport.TokenSummary
		if tracker != nil {
			ts := tracker.Summary()
			tokenSummary = &ts
		}

		slog.InfoContext(ctx, LogObservabilityComputed,
			slog.Any(circuit.LogKeyComponent, circuit.LogComponentBatch),
			slog.Any(circuit.LogKeyNodes, len(stepLatency)),
			slog.Any(circuit.LogKeyCount, len(errorRates)),
		)

		return &SessionRunResult{
			BatchResults: results,
			StepLatency:  stepLatency,
			ErrorRates:   errorRates,
			TokenSummary: tokenSummary,
		}, nil
	}
}

// SessionRunResult wraps BatchWalkResults with auto-computed observability
// data. Consumers that use SessionFactoryToConfig get this as their result
// type; consumers with custom RunFunc are unaffected.
type SessionRunResult struct {
	BatchResults []engine.BatchWalkResult `json:"batch_results,omitempty"`
	StepLatency  map[string]LatencyStats  `json:"step_latency,omitempty"`
	ErrorRates   map[string]NodeErrorRate `json:"error_rates,omitempty"`
	TokenSummary *agentport.TokenSummary  `json:"token_summary,omitempty"`
}

// LatencyStats holds per-node latency percentiles computed from walk events.
type LatencyStats struct {
	Count int           `json:"count"`
	Min   time.Duration `json:"min"`
	Max   time.Duration `json:"max"`
	Mean  time.Duration `json:"mean"`
	P50   time.Duration `json:"p50"`
	P95   time.Duration `json:"p95"`
	P99   time.Duration `json:"p99"`
}

// NodeErrorRate tracks error frequency for a single node across all cases.
type NodeErrorRate struct {
	TotalCases int     `json:"total_cases"`
	ErrorCount int     `json:"error_count"`
	ErrorRate  float64 `json:"error_rate"`
}

// computeStepLatency derives per-node latency percentiles from collected
// walk events. Only node_exit events carry elapsed durations.
func computeStepLatency(events []circuit.WalkEvent) map[string]LatencyStats {
	// Collect durations per node from node_exit events.
	durations := make(map[string][]time.Duration)
	for i := range events {
		e := &events[i]
		if e.Type == circuit.EventNodeExit && e.Elapsed > 0 {
			durations[e.Node] = append(durations[e.Node], e.Elapsed)
		}
	}

	result := make(map[string]LatencyStats, len(durations))
	for node, durs := range durations {
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })

		var total time.Duration
		for _, d := range durs {
			total += d
		}

		n := len(durs)
		result[node] = LatencyStats{
			Count: n,
			Min:   durs[0],
			Max:   durs[n-1],
			Mean:  total / time.Duration(n),
			P50:   percentile(durs, 50),
			P95:   percentile(durs, 95),
			P99:   percentile(durs, 99),
		}
	}
	return result
}

// percentile returns the p-th percentile from a sorted duration slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	// Linear interpolation between adjacent ranks.
	frac := rank - float64(lower)
	return sorted[lower] + time.Duration(frac*float64(sorted[upper]-sorted[lower]))
}

// computeErrorRates builds a per-node error rate from BatchWalk results.
// A node counts as "visited" if it appears in the case's path, and
// "errored" if the case errored AND the node was the last in its path
// (the node where execution failed).
func computeErrorRates(results []engine.BatchWalkResult) map[string]NodeErrorRate {
	type counts struct {
		total  int
		errors int
	}
	nodeStats := make(map[string]*counts)

	for i := range results {
		r := &results[i]
		// Count each visited node.
		for _, node := range r.Path {
			c, ok := nodeStats[node]
			if !ok {
				c = &counts{}
				nodeStats[node] = c
			}
			c.total++
		}
		// If the case errored, attribute the error to the last visited node.
		if r.Error != nil && len(r.Path) > 0 {
			lastNode := r.Path[len(r.Path)-1]
			if c, ok := nodeStats[lastNode]; ok {
				c.errors++
			}
		}
	}

	result := make(map[string]NodeErrorRate, len(nodeStats))
	for node, c := range nodeStats {
		rate := 0.0
		if c.total > 0 {
			rate = float64(c.errors) / float64(c.total)
		}
		result[node] = NodeErrorRate{
			TotalCases: c.total,
			ErrorCount: c.errors,
			ErrorRate:  rate,
		}
	}
	return result
}

func wrapWithACPWorkers(inner RunFunc, params StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) RunFunc {
	agentCmd, _ := params.Extra[ExtraKeyDispatchCommand].(string)
	if agentCmd == "" {
		agentCmd = defaultACPAgent
	}
	workers := max(params.Parallel, 1)

	return func(ctx context.Context) (any, error) {
		meter := agentport.NewInMemoryMeter()
		broker := agentport.NewBroker("",
			agentport.WithMeter(meter),
		)

		for range workers {
			_, spawnErr := broker.Spawn(ctx, agentport.ActorConfig{
				Model: agentCmd,
				Role:  defaultACPRole,
			})
			if spawnErr != nil {
				return nil, spawnErr
			}
		}

		var acpOpts []dispatch.ACPWorkerOption
		acpOpts = append(acpOpts,
			dispatch.WithACPWorkerBus(bus),
			dispatch.WithACPWorkerLogger(slog.Default()),
		)

		// Spawn a dialectic collective for hard steps (investigate, review).
		coll, collErr := agentport.SpawnCollective(ctx, broker, 2, &agentport.Dialectic{MaxRounds: 2}) //nolint:mnd // thesis + antithesis
		if collErr != nil {
			slog.WarnContext(ctx, circuit.LogCollectiveSpawnFailed, slog.Any(circuit.LogKeyError, collErr))
		} else {
			acpOpts = append(acpOpts, dispatch.WithACPWorkerCollective(coll))
		}

		acpDisp := dispatch.NewACPWorkerDispatcher(
			disp, broker, defaultACPRole, workers, acpOpts...,
		)
		go func() {
			if err := acpDisp.Run(ctx); err != nil {
				slog.ErrorContext(ctx, circuit.LogACPDispatchError, slog.Any(circuit.LogKeyError, err))
			}
		}()

		return inner(ctx)
	}
}

// collectHealthComponents gathers unique components with HealthChecker
// from the SessionConfig's Cases. Deduplicates by component name.
func collectHealthComponents(cfg *engine.SessionConfig) []*engine.Component {
	seen := make(map[string]bool)
	var result []*engine.Component
	for i := range cfg.Cases {
		for _, comp := range cfg.Cases[i].Components {
			if comp.Health != nil && !seen[comp.Name] {
				seen[comp.Name] = true
				result = append(result, comp)
			}
		}
	}
	return result
}

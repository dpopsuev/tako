package mcp

import (
	"context"
	"log/slog"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"
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
			engineParams := engine.SessionParams{
				Parallel:   params.Parallel,
				Force:      params.Force,
				Extra:      params.Extra,
				DomainFS:   params.DomainFS,
				StateDir:   params.StateDir,
				Observer:   params.Observer,
				Dispatcher: disp,
				Relayer:    &dispatch.MuxRelayer{Disp: disp},
			}

			sessionCfg, err := factory.CreateSession(ctx, &engineParams)
			if err != nil {
				return nil, SessionMeta{}, err
			}

			runFn := buildRunFunc(sessionCfg, &engineParams)

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

func buildRunFunc(cfg *engine.SessionConfig, params *engine.SessionParams) RunFunc {
	if cfg.RunFunc != nil {
		return cfg.RunFunc
	}
	return func(ctx context.Context) (any, error) {
		bwCfg := engine.BatchWalkConfig{
			Def: cfg.CircuitDef,
			Shared: &engine.GraphRegistries{
				Transformers: cfg.Transformers,
				Extractors:   cfg.Extractors,
				Hooks:        cfg.Hooks,
			},
			Cases:    cfg.Cases,
			Parallel: params.Parallel,
			Observer: params.Observer,
		}
		return engine.BatchWalk(ctx, bwCfg), nil
	}
}

func wrapWithACPWorkers(inner RunFunc, params StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) RunFunc {
	agentCmd, _ := params.Extra[ExtraKeyDispatchCommand].(string)
	if agentCmd == "" {
		agentCmd = defaultACPAgent
	}
	workers := max(params.Parallel, 1)

	return func(ctx context.Context) (any, error) {
		staff := agentport.NewStaff(agentport.NewACPLauncher())

		for range workers {
			_, spawnErr := staff.Spawn(ctx, defaultACPRole, agentport.LaunchConfig{
				Model: agentCmd,
			})
			if spawnErr != nil {
				staff.KillAll(ctx)
				return nil, spawnErr
			}
		}
		defer staff.KillAll(ctx)

		var acpOpts []dispatch.ACPWorkerOption
		acpOpts = append(acpOpts,
			dispatch.WithACPWorkerBus(bus),
			dispatch.WithACPWorkerLogger(slog.Default()),
		)

		// Spawn a dialectic collective for hard steps (investigate, review).
		// Two extra agents debate — the collective is asked instead of a
		// single worker for steps in collectiveSteps.
		coll, collErr := agentport.SpawnCollective(ctx, staff, agentport.CollectiveConfig{
			Role:     "debater",
			Strategy: &agentport.Dialectic{MaxRounds: 2},
			Agents: []agentport.LaunchConfig{
				{Role: "thesis", Model: agentCmd},
				{Role: "antithesis", Model: agentCmd},
			},
		})
		if collErr != nil {
			slog.WarnContext(ctx, "collective spawn failed, falling back to single-agent dispatch", "error", collErr)
		} else {
			acpOpts = append(acpOpts, dispatch.WithACPWorkerCollective(coll))
		}

		acpDisp := dispatch.NewACPWorkerDispatcher(
			disp, staff, defaultACPRole, workers, acpOpts...,
		)
		go func() {
			if err := acpDisp.Run(ctx); err != nil {
				slog.ErrorContext(ctx, "ACP worker dispatch error", "error", err)
			}
		}()

		return inner(ctx)
	}
}

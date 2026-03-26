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

// SessionHooksToConfig bridges engine.SessionHooks (new API) to
// CircuitConfig.CreateSession (old API). The framework creates
// dispatcher and bus internally, then builds a RunFunc from the
// consumer's SessionConfig.
func SessionHooksToConfig(hooks engine.SessionHooks) CircuitConfig {
	return CircuitConfig{
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

			sessionCfg, err := hooks.CreateSession(ctx, engineParams)
			if err != nil {
				return nil, SessionMeta{}, err
			}

			runFn := buildRunFunc(sessionCfg, engineParams)

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
		FormatReport: hooks.FormatReport,
		StepSchemas:  hooks.StepSchemas,
	}
}

func buildRunFunc(cfg *engine.SessionConfig, params engine.SessionParams) RunFunc {
	if cfg.RunFunc != nil {
		return cfg.RunFunc
	}
	return func(ctx context.Context) (any, error) {
		bwCfg := engine.BatchWalkConfig{
			Def: cfg.CircuitDef,
			Shared: engine.GraphRegistries{
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

		acpDisp := dispatch.NewACPWorkerDispatcher(
			disp, staff, defaultACPRole, workers,
			dispatch.WithACPWorkerBus(bus),
			dispatch.WithACPWorkerLogger(slog.Default()),
		)
		go acpDisp.Run(ctx)

		return inner(ctx)
	}
}

package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
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
				Parallel:         params.Parallel,
				Force:            params.Force,
				Extra:            params.Extra,
				DomainFS:         params.DomainFS,
				StateDir:         params.StateDir,
				Observer:         params.Observer,
				Dispatcher:       disp,
				Relayer:          &dispatch.MuxRelayer{Disp: disp},
				PromptStore:      params.PromptStore,
				ResourceRegistry: params.ResourceRegistry,
			}

			sessionCfg, err := factory.CreateSession(ctx, &engineParams)
			if err != nil {
				return nil, SessionMeta{}, err
			}

			// Run consumer preflight before launching the run goroutine.
			if sessionCfg.Preflight != nil {
				if err := sessionCfg.Preflight(ctx); err != nil {
					return nil, SessionMeta{}, fmt.Errorf("preflight: %w", err)
				}
			} else {
				slog.WarnContext(ctx, "SessionConfig.Preflight is nil — consider adding fail-fast validation")
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
		broker := agentport.NewBroker("")

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

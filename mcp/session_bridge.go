package mcp

import (
	"context"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"
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

			// When the consumer provides a RunFunc, use it directly —
			// this supports calibration pipelines that need the full
			// load → walk → collect → score → report cycle.
			var runFn RunFunc
			if sessionCfg.RunFunc != nil {
				runFn = sessionCfg.RunFunc
			} else {
				runFn = func(ctx context.Context) (any, error) {
					bwCfg := engine.BatchWalkConfig{
						Def: sessionCfg.CircuitDef,
						Shared: engine.GraphRegistries{
							Transformers: sessionCfg.Transformers,
							Extractors:   sessionCfg.Extractors,
							Hooks:        sessionCfg.Hooks,
						},
						Cases:    sessionCfg.Cases,
						Parallel: engineParams.Parallel,
						Observer: engineParams.Observer,
					}
					results := engine.BatchWalk(ctx, bwCfg)
					return results, nil
				}
			}

			_ = bus // framework-created, available via signal tools

			meta := SessionMeta{
				TotalCases: sessionCfg.Meta.TotalCases,
				Scenario:   sessionCfg.Meta.Scenario,
			}
			return runFn, meta, nil
		},
		FormatReport: hooks.FormatReport,
	}
}

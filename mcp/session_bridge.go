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
				Parallel: params.Parallel,
				Force:    params.Force,
				Extra:    params.Extra,
				Observer: params.Observer,
			}

			sessionCfg, err := hooks.CreateSession(ctx, engineParams)
			if err != nil {
				return nil, SessionMeta{}, err
			}

			// Framework builds the RunFunc — consumer never touches
			// dispatcher or bus.
			runFn := func(ctx context.Context) (any, error) {
				bwCfg := engine.BatchWalkConfig{
					Def:      sessionCfg.CircuitDef,
					Shared:   engine.GraphRegistries{
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

			_ = disp // framework-created, wired internally
			_ = bus  // framework-created, wired internally

			meta := SessionMeta{
				TotalCases: sessionCfg.Meta.TotalCases,
				Scenario:   sessionCfg.Meta.Scenario,
			}
			return runFn, meta, nil
		},
		FormatReport: hooks.FormatReport,
	}
}

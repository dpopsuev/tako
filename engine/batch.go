package engine

// Category: Execution

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dpopsuev/origami/core"
	"golang.org/x/sync/errgroup"
)

// BatchCase represents a single case in a batch walk.
type BatchCase struct {
	ID         string
	Context    map[string]any
	Components []*Component
}

// BatchWalkResult captures the outcome of walking one case.
type BatchWalkResult struct {
	CaseID        string
	Path          []string
	StepArtifacts map[string]core.Artifact
	State         *core.WalkerState
	Error         error
}

// BatchWalkConfig configures a batch walk over a circuit.
type BatchWalkConfig struct {
	Def            *CircuitDef
	Shared         GraphRegistries
	Cases          []BatchCase
	Parallel       int
	OnCaseComplete func(index int, result BatchWalkResult)
	Observer       core.WalkObserver // external observer, composed with internal path/artifact collector
}

// BatchWalk walks a circuit once per case, optionally in parallel.
func BatchWalk(ctx context.Context, cfg BatchWalkConfig) []BatchWalkResult {
	results := make([]BatchWalkResult, len(cfg.Cases))

	walkOne := func(ctx context.Context, i int, bc BatchCase) {
		reg := cfg.Shared
		if len(bc.Components) > 0 {
			var err error
			reg, err = MergeComponents(reg, bc.Components...)
			if err != nil {
				results[i] = BatchWalkResult{CaseID: bc.ID, Error: err}
				return
			}
		}

		runner, err := NewRunnerWith(cfg.Def, reg)
		if err != nil {
			results[i] = BatchWalkResult{CaseID: bc.ID, Error: err}
			return
		}

		walker := core.NewProcessWalker(bc.ID)
		walker.State().MergeContext(bc.Context)

		var mu sync.Mutex
		var path []string
		stepArtifacts := map[string]core.Artifact{}

		obs := core.WalkObserverFunc(func(e core.WalkEvent) {
			mu.Lock()
			defer mu.Unlock()
			if e.Type == core.EventNodeEnter {
				path = append(path, e.Node)
			}
			if e.Type == core.EventNodeExit && e.Artifact != nil {
				stepArtifacts[e.Node] = e.Artifact
			}
		})
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			if cfg.Observer != nil {
				dg.SetObserver(core.MultiObserver{obs, cfg.Observer})
			} else {
				dg.SetObserver(obs)
			}
		}

		walkErr := runner.Walk(ctx, walker, cfg.Def.Start)
		if walkErr != nil {
			slog.Warn("case walk failed", "component", "batch_walk", "case_id", bc.ID, "error", walkErr)
		}

		results[i] = BatchWalkResult{
			CaseID:        bc.ID,
			Path:          path,
			StepArtifacts: stepArtifacts,
			State:         walker.State(),
			Error:         walkErr,
		}
	}

	if cfg.Parallel > 1 {
		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(cfg.Parallel)
		for i, bc := range cfg.Cases {
			i, bc := i, bc
			g.Go(func() error {
				walkOne(gCtx, i, bc)
				if cfg.OnCaseComplete != nil {
					cfg.OnCaseComplete(i, results[i])
				}
				return nil
			})
		}
		_ = g.Wait()
	} else {
		for i, bc := range cfg.Cases {
			walkOne(ctx, i, bc)
			if cfg.OnCaseComplete != nil {
				cfg.OnCaseComplete(i, results[i])
			}
		}
	}

	return results
}

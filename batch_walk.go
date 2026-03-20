package framework

// Category: Execution

import (
	"context"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
)

// BatchCase represents a single case in a batch walk.
type BatchCase struct {
	ID       string
	Context  map[string]any
	Components []*Component
}

// BatchWalkResult captures the outcome of walking one case.
type BatchWalkResult struct {
	CaseID        string
	Path          []string
	StepArtifacts map[string]Artifact
	State         *WalkerState
	Error         error
}

// BatchWalkConfig configures a batch walk over a circuit.
type BatchWalkConfig struct {
	Def            *CircuitDef
	Shared         GraphRegistries
	Cases          []BatchCase
	Parallel       int
	OnCaseComplete func(index int, result BatchWalkResult)
	Observer       WalkObserver // external observer, composed with internal path/artifact collector
}

// BatchWalk walks a circuit once per case, optionally in parallel.
// Each case gets its own runner (shared registries + per-case components),
// walker, and observer. Results are returned in case order.
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

		walker := NewProcessWalker(bc.ID)
		walker.State().MergeContext(bc.Context)

		// Set case ID on TraceRecorder if the observer supports it.
		if tr, ok := cfg.Observer.(interface{ SetCaseID(string) }); ok {
			tr.SetCaseID(bc.ID)
		}

		var mu sync.Mutex
		var path []string
		stepArtifacts := map[string]Artifact{}

		obs := WalkObserverFunc(func(e WalkEvent) {
			mu.Lock()
			defer mu.Unlock()
			if e.Type == EventNodeEnter {
				path = append(path, e.Node)
			}
			if e.Type == EventNodeExit && e.Artifact != nil {
				stepArtifacts[e.Node] = e.Artifact
			}
		})
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			if cfg.Observer != nil {
				dg.SetObserver(MultiObserver{obs, cfg.Observer})
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

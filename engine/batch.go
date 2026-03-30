package engine

// Category: Execution

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dpopsuev/origami/circuit"
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
	StepArtifacts map[string]circuit.Artifact
	State         *circuit.WalkerState
	Error         error
}

// BatchWalkConfig configures a batch walk over a circuit.
type BatchWalkConfig struct {
	Def            *circuit.CircuitDef
	Shared         *GraphRegistries
	Cases          []BatchCase
	Parallel       int
	OnCaseComplete func(index int, result BatchWalkResult)
	Observer       circuit.WalkObserver // external observer, composed with internal path/artifact collector
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

		walker := circuit.NewProcessWalker(bc.ID)
		walker.State().MergeContext(bc.Context)

		var mu sync.Mutex
		var path []string
		stepArtifacts := map[string]circuit.Artifact{}

		obs := circuit.WalkObserverFunc(func(e *circuit.WalkEvent) {
			mu.Lock()
			defer mu.Unlock()
			if e.Type == circuit.EventNodeEnter {
				path = append(path, e.Node)
			}
			if e.Type == circuit.EventNodeExit && e.Artifact != nil {
				stepArtifacts[e.Node] = e.Artifact
			}
		})
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			if cfg.Observer != nil {
				dg.SetObserver(circuit.MultiObserver{obs, cfg.Observer})
			} else {
				dg.SetObserver(obs)
			}
		}

		walkErr := runner.Walk(ctx, walker, string(cfg.Def.Start))
		if walkErr != nil {
			slog.WarnContext(ctx, circuit.LogCaseWalkFailed, slog.String(circuit.LogKeyComponent, circuit.LogComponentBatch), slog.String(circuit.LogKeyCaseID, bc.ID), slog.Any(circuit.LogKeyError, walkErr))
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

	DiagnoseNodeCoverage(cfg.Def, results)

	return results
}

// DiagnoseNodeCoverage checks which declared nodes were never visited by
// any case in the batch. Emits a warning for each unvisited node.
// This catches overlay nodes that resolve but are never walked (e.g.,
// gather-code skipped due to sparse registries or routing issues).
func DiagnoseNodeCoverage(def *circuit.CircuitDef, results []BatchWalkResult) []string {
	if def == nil || len(results) == 0 {
		return nil
	}

	// Collect all visited nodes across all cases.
	visited := make(map[string]bool)
	for _, r := range results {
		for _, node := range r.Path {
			visited[node] = true
		}
	}

	// Compare against declared nodes (skip "done" — it's a virtual terminal).
	var unvisited []string
	for i := range def.Nodes {
		name := string(def.Nodes[i].Name)
		if name == string(def.Done) {
			continue
		}
		if !visited[name] {
			unvisited = append(unvisited, name)
		}
	}

	if len(unvisited) > 0 {
		slog.WarnContext(context.Background(), circuit.LogUnvisitedNodes,
			slog.Any(circuit.LogKeyComponent, circuit.LogComponentBatch),
			slog.Any(circuit.LogKeyNodes, unvisited),
			slog.Any(circuit.LogKeyCount, len(unvisited)),
			slog.Any(circuit.LogKeyTotalCases, len(results)),
			slog.Any(circuit.LogKeyCircuit, def.Circuit))
	}

	return unvisited
}

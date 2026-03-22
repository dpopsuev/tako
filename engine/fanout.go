package engine

// Category: Execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/origami/core"
	"golang.org/x/sync/errgroup"
)

// parallelMatch pairs a matched edge with its transition during fan-out detection.
type parallelMatch struct {
	edge       core.Edge
	transition *core.Transition
}

// isParallelEdge returns true if the edge implements ParallelEdge and is marked parallel.
func isParallelEdge(e core.Edge) bool {
	if pe, ok := e.(core.ParallelEdge); ok {
		return pe.IsParallel()
	}
	return false
}

// walkFanOut executes parallel branch nodes concurrently and returns the merge node.
func (g *DefaultGraph) walkFanOut(
	ctx context.Context,
	walker core.Walker,
	obs core.WalkObserver,
	sourceNode core.Node,
	sourceArtifact core.Artifact,
	matches []parallelMatch,
) (string, error) {
	state := walker.State()
	walkerName := walker.Identity().PersonaName

	branchNames := make([]string, len(matches))
	for i, m := range matches {
		branchNames[i] = m.transition.NextNode
	}
	emitEvent(obs, core.WalkEvent{
		Type:     core.EventFanOutStart,
		Node:     sourceNode.Name(),
		Walker:   walkerName,
		Metadata: map[string]any{"branches": branchNames},
	})

	results := make([]branchResult, len(matches))
	var outputMu sync.Mutex

	eg, egCtx := errgroup.WithContext(ctx)

	for i, m := range matches {
		targetNode, ok := g.nodeIndex[m.transition.NextNode]
		if !ok {
			return "", fmt.Errorf("%w: fan-out target %q from edge %s",
				core.ErrNodeNotFound, m.transition.NextNode, m.edge.ID())
		}

		eg.Go(func() error {
			emitEvent(obs, core.WalkEvent{Type: core.EventNodeEnter, Node: targetNode.Name(), Walker: walkerName})
			start := time.Now()

			nc := core.NodeContext{
				WalkerState:   state,
				PriorArtifact: sourceArtifact,
				Meta:          make(map[string]any),
			}

			branchCtx, branchCancel := g.nodeCtx(egCtx, targetNode.Name())
			defer branchCancel()

			art, err := walker.Handle(branchCtx, targetNode, nc)
			elapsed := time.Since(start)

			if err != nil {
				emitEvent(obs, core.WalkEvent{
					Type: core.EventNodeExit, Node: targetNode.Name(),
					Walker: walkerName, Elapsed: elapsed, Error: err,
				})
				return fmt.Errorf("node %s: %w", targetNode.Name(), err)
			}

			emitEvent(obs, core.WalkEvent{
				Type: core.EventNodeExit, Node: targetNode.Name(),
				Walker: walkerName, Artifact: art, Elapsed: elapsed,
			})

			outputMu.Lock()
			if state.Outputs == nil {
				state.Outputs = make(map[string]core.Artifact)
			}
			state.Outputs[targetNode.Name()] = art
			outputMu.Unlock()

			results[i] = branchResult{nodeName: targetNode.Name(), artifact: art}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		state.Status = "error"
		emitEvent(obs, core.WalkEvent{Type: core.EventWalkError, Node: sourceNode.Name(), Error: err})
		return "", err
	}

	for _, m := range matches {
		state.RecordStep(sourceNode.Name(), m.edge.ID(), m.edge.ID(),
			time.Now().UTC().Format(time.RFC3339))
	}

	mergeNodeName, err := g.findMergeTarget(results, state)
	if err != nil {
		state.Status = "error"
		emitEvent(obs, core.WalkEvent{Type: core.EventWalkError, Node: sourceNode.Name(), Error: err})
		return "", err
	}

	emitEvent(obs, core.WalkEvent{
		Type:     core.EventFanOutEnd,
		Node:     sourceNode.Name(),
		Walker:   walkerName,
		Metadata: map[string]any{"merge": mergeNodeName},
	})

	return mergeNodeName, nil
}

// findMergeTarget evaluates outgoing edges from each parallel branch and returns
// the common successor node.
func (g *DefaultGraph) findMergeTarget(results []branchResult, state *core.WalkerState) (string, error) {
	var mergeNodeName string

	for _, r := range results {
		edges := g.EdgesFrom(r.nodeName)
		var found string
		for _, e := range edges {
			t := e.Evaluate(r.artifact, state)
			if t != nil {
				found = t.NextNode
				break
			}
		}
		if found == "" {
			return "", fmt.Errorf("%w: branch %q has no matching outgoing edge",
				core.ErrFanOutMerge, r.nodeName)
		}
		if mergeNodeName == "" {
			mergeNodeName = found
		} else if mergeNodeName != found {
			return "", fmt.Errorf("%w: branches disagree on merge target: %q vs %q",
				core.ErrFanOutMerge, mergeNodeName, found)
		}
	}

	if mergeNodeName == "" {
		return "", fmt.Errorf("%w: no merge node found", core.ErrFanOutMerge)
	}

	return mergeNodeName, nil
}

// branchResult holds the output of a single parallel branch.
type branchResult struct {
	nodeName string
	artifact core.Artifact
}

// ListArtifact wraps multiple artifacts from parallel branches into a single
// composite artifact.
type ListArtifact struct {
	Items []core.Artifact
}

func (a *ListArtifact) Type() string       { return "list" }
func (a *ListArtifact) Confidence() float64 { return 0 }
func (a *ListArtifact) Raw() any            { return a.Items }

// applyMergeStrategy combines branch results into a single merged artifact.
func applyMergeStrategy(strategy string, results []branchResult) core.Artifact {
	if len(results) == 0 {
		return nil
	}
	switch strategy {
	case MergeAppend:
		items := make([]core.Artifact, 0, len(results))
		for _, r := range results {
			if r.artifact != nil {
				items = append(items, r.artifact)
			}
		}
		return &ListArtifact{Items: items}
	case MergeLatest:
		return results[len(results)-1].artifact
	default:
		return results[0].artifact
	}
}

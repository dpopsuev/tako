package engine

// Category: Execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"golang.org/x/sync/errgroup"
)

// parallelMatch pairs a matched edge with its transition during fan-out detection.
type parallelMatch struct {
	edge       circuit.Edge
	transition *circuit.Transition
}

// MergeEdge is an optional interface for edges that expose a fan-in merge strategy.
// When parallel edges carry a merge strategy, it is used to combine branch results
// before passing the merged artifact to the merge node.
type MergeEdge interface {
	MergeStrategy() string
}

// isParallelEdge returns true if the edge implements ParallelEdge and is marked parallel.
func isParallelEdge(e circuit.Edge) bool {
	if pe, ok := e.(circuit.ParallelEdge); ok {
		return pe.IsParallel()
	}
	return false
}

// edgeMergeStrategy extracts the merge strategy from an edge if it implements MergeEdge.
// Returns empty string if the edge does not expose a merge strategy.
func edgeMergeStrategy(e circuit.Edge) string {
	if me, ok := e.(MergeEdge); ok {
		return me.MergeStrategy()
	}
	return ""
}

// walkFanOut executes parallel branch nodes concurrently and returns the merge
// node name together with a merged artifact produced by applying the fan-in
// merge strategy declared on the parallel edges.
func (g *DefaultGraph) walkFanOut(
	ctx context.Context,
	walker circuit.Walker,
	obs circuit.WalkObserver,
	sourceNode circuit.Node,
	sourceArtifact circuit.Artifact,
	matches []parallelMatch,
) (string, circuit.Artifact, error) {
	state := walker.State()
	walkerName := walker.Identity().Name

	branchNames := make([]string, len(matches))
	for i, m := range matches {
		branchNames[i] = m.transition.NextNode
	}
	emitEvent(obs, &circuit.WalkEvent{
		Type:     circuit.EventFanOutStart,
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
			return "", nil, fmt.Errorf("%w: fan-out target %q from edge %s",
				circuit.ErrNodeNotFound, m.transition.NextNode, m.edge.ID())
		}

		eg.Go(func() error {
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: targetNode.Name(), Walker: walkerName})
			start := time.Now()

			nc := circuit.NodeContext{
				WalkerState:   state,
				PriorArtifact: sourceArtifact,
				Meta:          make(map[string]any),
			}

			branchCtx, branchCancel := g.nodeCtx(egCtx, targetNode.Name())
			defer branchCancel()

			art, err := walker.Handle(branchCtx, targetNode, nc)
			elapsed := time.Since(start)

			if err != nil {
				emitEvent(obs, &circuit.WalkEvent{
					Type: circuit.EventNodeExit, Node: targetNode.Name(),
					Walker: walkerName, Elapsed: elapsed, Error: err,
				})
				return fmt.Errorf("node %s: %w", targetNode.Name(), err)
			}

			emitEvent(obs, &circuit.WalkEvent{
				Type: circuit.EventNodeExit, Node: targetNode.Name(),
				Walker: walkerName, Artifact: art, Elapsed: elapsed,
			})

			outputMu.Lock()
			if state.Outputs == nil {
				state.Outputs = make(map[string]circuit.Artifact)
			}
			state.Outputs[targetNode.Name()] = art
			outputMu.Unlock()

			results[i] = branchResult{nodeName: targetNode.Name(), artifact: art}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		state.Status = walkStatusError
		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: sourceNode.Name(), Error: err})
		return "", nil, err
	}

	for _, m := range matches {
		state.RecordStep(sourceNode.Name(), m.edge.ID(), m.edge.ID(),
			time.Now().UTC().Format(time.RFC3339))
	}

	mergeNodeName, err := g.findMergeTarget(results, state)
	if err != nil {
		state.Status = walkStatusError
		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: sourceNode.Name(), Error: err})
		return "", nil, err
	}

	// Determine the merge strategy from the parallel edges. Use the first
	// non-empty Merge value; default to "append" when none is specified.
	strategy := determineMergeStrategy(matches)
	merged := applyMergeStrategy(strategy, results)

	emitEvent(obs, &circuit.WalkEvent{
		Type:     circuit.EventFanOutEnd,
		Node:     sourceNode.Name(),
		Walker:   walkerName,
		Artifact: merged,
		Metadata: map[string]any{"merge": mergeNodeName, "strategy": strategy},
	})

	return mergeNodeName, merged, nil
}

// determineMergeStrategy inspects the parallel edges and returns the first
// non-empty Merge value. Falls back to MergeAppend when no edge declares a
// strategy.
func determineMergeStrategy(matches []parallelMatch) string {
	for _, m := range matches {
		if s := edgeMergeStrategy(m.edge); s != "" {
			return s
		}
	}
	return MergeAppend
}

// findMergeTarget evaluates outgoing edges from each parallel branch and returns
// the common successor node.
func (g *DefaultGraph) findMergeTarget(results []branchResult, state *circuit.WalkerState) (string, error) {
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
				circuit.ErrFanOutMerge, r.nodeName)
		}
		if mergeNodeName == "" {
			mergeNodeName = found
		} else if mergeNodeName != found {
			return "", fmt.Errorf("%w: branches disagree on merge target: %q vs %q",
				circuit.ErrFanOutMerge, mergeNodeName, found)
		}
	}

	if mergeNodeName == "" {
		return "", fmt.Errorf("%w: no merge node found", circuit.ErrFanOutMerge)
	}

	return mergeNodeName, nil
}

// branchResult holds the output of a single parallel branch.
type branchResult struct {
	nodeName string
	artifact circuit.Artifact
}

// ListArtifact wraps multiple artifacts from parallel branches into a single
// composite artifact.
type ListArtifact struct {
	Items []circuit.Artifact
}

const artifactTypeList = "list"

func (a *ListArtifact) Type() string        { return artifactTypeList }
func (a *ListArtifact) Confidence() float64 { return 0 }
func (a *ListArtifact) Raw() any            { return a.Items }

// applyMergeStrategy combines branch results into a single merged artifact.
func applyMergeStrategy(strategy string, results []branchResult) circuit.Artifact {
	if len(results) == 0 {
		return nil
	}
	switch strategy {
	case MergeAppend:
		items := make([]circuit.Artifact, 0, len(results))
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

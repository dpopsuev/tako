package engine

// Category: Execution — DefaultGraph implementation.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

const (
	walkStatusError       = "error"
	walkStatusDone        = "done"
	walkStatusInterrupted = "interrupted"
)

// errWalkInterrupted is the sentinel error returned by Walk when a node
// signals an Interrupt. The Run() function checks for this to decide
// whether to clean up the checkpoint.
var errWalkInterrupted = fmt.Errorf("walk interrupted")

// Graph is a directed graph of Nodes connected by Edges, partitioned into Zones.
type Graph interface {
	Name() string
	Nodes() []circuit.Node
	Edges() []circuit.Edge
	Zones() []Zone
	NodeByName(name string) (circuit.Node, bool)
	EdgesFrom(nodeName string) []circuit.Edge
	Walk(ctx context.Context, walker circuit.Walker, startNode string) error
	WalkTeam(ctx context.Context, team *Team, startNode string) error
}

// Zone is a meta-phase grouping of Nodes with shared characteristics.
type Zone struct {
	Name            string
	NodeNames       []string
	ElementAffinity circuit.Element
	Stickiness      int // 0-3 stickiness value for agents in this zone
	Domain          string
	ContextFilter   *circuit.ContextFilterDef
}

// circuit.ContextFilterDef configures which context keys survive a zone transition.
// Imported from circuit/ via alias.

// DefaultGraph is the reference Graph implementation. It stores nodes and
// edges in maps for O(1) lookup while preserving edge definition order
// for deterministic first-match evaluation.
type DefaultGraph struct {
	name         string
	nodes        []circuit.Node
	edges        []circuit.Edge
	zones        []Zone
	nodeIndex    map[string]circuit.Node
	edgeIndex    map[string][]circuit.Edge // from-node -> edges in definition order
	nodeTimeouts map[string]time.Duration  // per-node timeout (from DSL)
	doneNode     string                    // terminal pseudo-node name (walk stops here)
	observer     circuit.WalkObserver      // graph-level observer, used by Walk and composed with team observer in WalkTeam
	registries   *GraphRegistries          // retained for DelegateNode sub-walk building
}

// GraphOption configures a DefaultGraph during construction.
type GraphOption func(*DefaultGraph)

// WithDoneNode sets the terminal pseudo-node name. When a transition targets
// this node, the walk completes successfully. Defaults to "_done".
func WithDoneNode(name string) GraphOption {
	return func(g *DefaultGraph) {
		g.doneNode = name
	}
}

// WithObserver attaches a graph-level observer that receives walk events
// from both Walk() and WalkTeam(). In WalkTeam(), this observer is composed
// with the team's observer via MultiObserver.
func WithObserver(obs circuit.WalkObserver) GraphOption {
	return func(g *DefaultGraph) {
		g.observer = obs
	}
}

// WithNodeTimeouts sets per-node timeout durations. When a node with a timeout
// is encountered during Walk, a derived context.WithTimeout is created so the
// node's Process (or delegate sub-walk) is canceled if it exceeds the budget.
func WithNodeTimeouts(m map[string]time.Duration) GraphOption {
	return func(g *DefaultGraph) {
		g.nodeTimeouts = m
	}
}

// SetObserver replaces the graph-level observer. This is useful for consumers
// that build graphs via NewRunnerWith and need to attach observers externally.
func (g *DefaultGraph) SetObserver(obs circuit.WalkObserver) {
	g.observer = obs
}

// SetRegistries sets the graph registries (needed for delegate sub-walk building).
// Exported for test backward compatibility.
func (g *DefaultGraph) SetRegistries(reg *GraphRegistries) {
	g.registries = reg
}

// NewGraph constructs a DefaultGraph from the provided nodes, edges, and zones.
// Returns an error if referential integrity checks fail (e.g. an edge
// references a nonexistent node).
func NewGraph(name string, nodes []circuit.Node, edges []circuit.Edge, zones []Zone, opts ...GraphOption) (*DefaultGraph, error) {
	g := &DefaultGraph{
		name:      name,
		nodes:     nodes,
		edges:     edges,
		zones:     zones,
		nodeIndex: make(map[string]circuit.Node, len(nodes)),
		edgeIndex: make(map[string][]circuit.Edge),
		doneNode:  "_done",
	}
	for _, opt := range opts {
		opt(g)
	}

	for _, n := range nodes {
		g.nodeIndex[n.Name()] = n
	}
	for _, e := range edges {
		if _, ok := g.nodeIndex[e.From()]; !ok {
			return nil, fmt.Errorf("%w: edge %s references source %q", circuit.ErrNodeNotFound, e.ID(), e.From())
		}
		to := e.To()
		if to != g.doneNode {
			if _, ok := g.nodeIndex[to]; !ok {
				return nil, fmt.Errorf("%w: edge %s references target %q", circuit.ErrNodeNotFound, e.ID(), to)
			}
		}
		g.edgeIndex[e.From()] = append(g.edgeIndex[e.From()], e)
	}

	return g, nil
}

func (g *DefaultGraph) Name() string          { return g.name }
func (g *DefaultGraph) Nodes() []circuit.Node { return g.nodes }
func (g *DefaultGraph) Edges() []circuit.Edge { return g.edges }
func (g *DefaultGraph) Zones() []Zone         { return g.zones }

func (g *DefaultGraph) NodeByName(name string) (circuit.Node, bool) {
	n, ok := g.nodeIndex[name]
	return n, ok
}

func (g *DefaultGraph) EdgesFrom(nodeName string) []circuit.Edge {
	return g.edgeIndex[nodeName]
}

// Walk traverses the graph starting at startNode using the provided walker.
// At each node, the walker processes the node to produce an artifact, then
// edges from that node are evaluated in definition order (first match wins).
// The walk completes when a transition targets the done node, or returns an
// error if no edge matches or a node is not found.
//
// If a graph-level observer is set via WithObserver, walk events are emitted
// at the same points as WalkTeam (node enter/exit, transitions, completion, errors).
//
//nolint:gocyclo,funlen // core graph traversal state machine — complexity is inherent
func (g *DefaultGraph) Walk(ctx context.Context, walker circuit.Walker, startNode string) error {
	obs := g.observer
	walkerName := walker.Identity().PersonaName

	node, ok := g.nodeIndex[startNode]
	if !ok {
		err := fmt.Errorf("%w: start node %q", circuit.ErrNodeNotFound, startNode)
		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: startNode, Error: err})
		return err
	}

	state := walker.State()
	state.CurrentNode = startNode
	var priorArtifact circuit.Artifact

	for {
		if err := ctx.Err(); err != nil {
			state.Status = walkStatusError
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Error: err})
			return err
		}

		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: node.Name(), Walker: walkerName})
		slog.DebugContext(ctx, circuit.LogNodeEnter, circuit.LogKeyComponent, circuit.LogComponentWalk, circuit.LogKeyNode, node.Name(), circuit.LogKeyWalker, walkerName)
		nodeStart := time.Now()

		nc := circuit.NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		nodeCtx, nodeCancel := g.nodeCtx(ctx, node.Name())

		var artifact circuit.Artifact
		var err error
		if dn, isDel := node.(DelegateNode); isDel {
			artifact, err = g.walkDelegate(nodeCtx, walker, obs, dn, nc)
		} else {
			artifact, err = walker.Handle(nodeCtx, node, nc)
		}
		nodeCancel()
		nodeElapsed := time.Since(nodeStart)

		if err != nil {
			if intr, ok := AsInterrupt(err); ok {
				state.Status = walkStatusInterrupted
				if intr.Data != nil {
					state.Context["interrupt_data"] = intr.Data
				}
				emitEvent(obs, &circuit.WalkEvent{
					Type:   circuit.EventWalkInterrupted,
					Node:   node.Name(),
					Walker: walkerName,
					Metadata: map[string]any{
						"reason": intr.Reason,
					},
				})
				return errWalkInterrupted
			}
			state.Status = walkStatusError
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventNodeExit, Node: node.Name(), Walker: walkerName, Elapsed: nodeElapsed, Error: err})
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		exitMeta := map[string]any{}
		if ca, ok := artifact.(circuit.CountableArtifact); ok {
			exitMeta["snr"] = evidenceSNR(ca.InputCount(), ca.OutputCount())
		}
		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventNodeExit, Node: node.Name(), Walker: walkerName, Artifact: artifact, Elapsed: nodeElapsed, Metadata: exitMeta})
		slog.DebugContext(ctx, circuit.LogNodeExit, circuit.LogKeyComponent, circuit.LogComponentWalk, circuit.LogKeyNode, node.Name(), circuit.LogKeyElapsed, nodeElapsed.Milliseconds())

		if artifact != nil && artifact.Confidence() > 0 {
			state.RecordConfidence(artifact.Confidence())
		}

		if state.Outputs == nil {
			state.Outputs = make(map[string]circuit.Artifact)
		}
		state.Outputs[node.Name()] = artifact

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = walkStatusDone
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkComplete, Node: node.Name(), Walker: walkerName})
			return nil
		}

		// Evaluate all edges, separating parallel from sequential matches.
		// If 2+ parallel edges match, fan-out to concurrent execution.
		var parallelMatches []parallelMatch
		var seqMatch *circuit.Transition
		var seqEdge circuit.Edge
		for _, e := range edges {
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventEdgeEvaluate, Node: node.Name(), Edge: e.ID(), Walker: walkerName})
			t := e.Evaluate(artifact, state)
			if t == nil {
				continue
			}
			if isParallelEdge(e) {
				parallelMatches = append(parallelMatches, parallelMatch{edge: e, transition: t})
			} else if seqMatch == nil {
				seqMatch = t
				seqEdge = e
			}
		}

		if len(parallelMatches) >= 2 {
			mergeNodeName, mergedArtifact, err := g.walkFanOut(ctx, walker, obs, node, artifact, parallelMatches)
			if err != nil {
				return err
			}
			if mergeNodeName == g.doneNode {
				state.Status = walkStatusDone
				emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkComplete, Walker: walkerName})
				return nil
			}
			nextNode, ok := g.nodeIndex[mergeNodeName]
			if !ok {
				state.Status = walkStatusError
				err := fmt.Errorf("%w: merge target %q", circuit.ErrNodeNotFound, mergeNodeName)
				emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Error: err, Walker: walkerName})
				return err
			}
			priorArtifact = mergedArtifact
			node = nextNode
			state.CurrentNode = mergeNodeName
			continue
		}

		// Sequential: use first sequential match, or single parallel edge
		matched := seqMatch
		matchedEdge := seqEdge
		if matched == nil && len(parallelMatches) == 1 {
			matched = parallelMatches[0].transition
			matchedEdge = parallelMatches[0].edge
		}

		if matched == nil {
			state.Status = walkStatusError
			err := fmt.Errorf("%w: node %q, artifact type %q", circuit.ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventTransition, Node: node.Name(), Edge: matchedEdge.ID(), Walker: walkerName})
		slog.DebugContext(ctx, circuit.LogEdgeTaken, circuit.LogKeyComponent, circuit.LogComponentWalk, circuit.LogKeyFrom, node.Name(), circuit.LogKeyEdge, matchedEdge.ID(), circuit.LogKeyTo, matched.NextNode, circuit.LogKeyLoop, matchedEdge.IsLoop(), circuit.LogKeyShortcut, matchedEdge.IsShortcut())

		if matchedEdge.IsLoop() {
			state.IncrementLoop(node.Name())
			slog.DebugContext(ctx, circuit.LogLoopIncremented, circuit.LogKeyComponent, circuit.LogComponentWalk, circuit.LogKeyNode, node.Name(), circuit.LogKeyCount, state.LoopCounts[node.Name()])
		}

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		fromZone := zoneForNode(node.Name(), g.zones)
		toZone := zoneForNode(matched.NextNode, g.zones)
		if fromZone != nil && (toZone == nil || fromZone.Name != toZone.Name) {
			applyContextFilter(state.Context, fromZone.ContextFilter)
		}

		if matched.NextNode == g.doneNode {
			state.Status = walkStatusDone
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkComplete, Walker: walkerName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = walkStatusError
			err := fmt.Errorf("%w: transition target %q from edge %s", circuit.ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Error: err})
			return err
		}

		priorArtifact = artifact
		node = nextNode
		state.CurrentNode = matched.NextNode
	}
}

// WalkTeam traverses the graph with multiple walkers coordinated by a
// scheduler. Before each node, the scheduler picks the walker. The
// observer (if non-nil) receives events for the full walk lifecycle.
// MaxSteps > 0 provides defense-in-depth against infinite loops.
//
// When both a graph-level observer (WithObserver) and team.Observer are set,
// events are fanned out to both via MultiObserver.
//
//nolint:gocyclo,funlen // multi-walker graph traversal — complexity is inherent
func (g *DefaultGraph) WalkTeam(ctx context.Context, team *Team, startNode string) error {
	obs := composeObservers(g.observer, team.Observer)

	node, ok := g.nodeIndex[startNode]
	if !ok {
		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: startNode, Error: fmt.Errorf("%w: start node %q", circuit.ErrNodeNotFound, startNode)})
		return fmt.Errorf("%w: start node %q", circuit.ErrNodeNotFound, startNode)
	}

	if len(team.Walkers) == 0 {
		return ErrTeamNoWalkers
	}

	var priorWalker circuit.Walker
	var priorArtifact circuit.Artifact
	steps := 0

	for {
		if err := ctx.Err(); err != nil {
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Error: err})
			return err
		}

		if team.MaxSteps > 0 && steps >= team.MaxSteps {
			err := fmt.Errorf("%w (%d) at node %q", ErrMaxStepsExceeded, team.MaxSteps, node.Name())
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		zone := zoneForNode(node.Name(), g.zones)
		walker := team.Scheduler.Select(SchedulerContext{
			Node:        node,
			Zone:        zone,
			Walkers:     team.Walkers,
			PriorWalker: priorWalker,
		})

		if priorWalker == nil || walker.Identity().PersonaName != priorWalker.Identity().PersonaName {
			meta := map[string]any{}
			if as, ok := team.Scheduler.(*AffinityScheduler); ok {
				meta["mismatch"] = as.LastMismatch()
			}
			emitEvent(obs, &circuit.WalkEvent{
				Type:     circuit.EventWalkerSwitch,
				Node:     node.Name(),
				Walker:   walker.Identity().PersonaName,
				Metadata: meta,
			})
		}

		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: node.Name(), Walker: walker.Identity().PersonaName})
		nodeStart := time.Now()

		state := walker.State()
		state.CurrentNode = node.Name()

		nc := circuit.NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		nodeCtx, nodeCancel := g.nodeCtx(ctx, node.Name())

		var artifact circuit.Artifact
		var err error
		if dn, isDel := node.(DelegateNode); isDel {
			artifact, err = g.walkDelegate(nodeCtx, walker, obs, dn, nc)
		} else {
			artifact, err = walker.Handle(nodeCtx, node, nc)
		}
		nodeCancel()
		nodeElapsed := time.Since(nodeStart)

		if err != nil {
			state.Status = walkStatusError
			emitEvent(obs, &circuit.WalkEvent{
				Type:    circuit.EventNodeExit,
				Node:    node.Name(),
				Walker:  walker.Identity().PersonaName,
				Elapsed: nodeElapsed,
				Error:   err,
			})
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		teamExitMeta := map[string]any{}
		if ca, ok := artifact.(circuit.CountableArtifact); ok {
			teamExitMeta["snr"] = evidenceSNR(ca.InputCount(), ca.OutputCount())
		}
		emitEvent(obs, &circuit.WalkEvent{
			Type:     circuit.EventNodeExit,
			Node:     node.Name(),
			Walker:   walker.Identity().PersonaName,
			Artifact: artifact,
			Elapsed:  nodeElapsed,
			Metadata: teamExitMeta,
		})

		if artifact != nil && artifact.Confidence() > 0 {
			state.RecordConfidence(artifact.Confidence())
		}

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = walkStatusDone
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkComplete, Node: node.Name(), Walker: walker.Identity().PersonaName})
			return nil
		}

		var matched *circuit.Transition
		var matchedEdge circuit.Edge
		for _, e := range edges {
			wName := walker.Identity().PersonaName
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventEdgeEvaluate, Node: node.Name(), Edge: e.ID(), Walker: wName})
			t := e.Evaluate(artifact, state)
			if t != nil {
				matched = t
				matchedEdge = e
				break
			}
		}

		if matched == nil {
			state.Status = walkStatusError
			err := fmt.Errorf("%w: node %q, artifact type %q", circuit.ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Node: node.Name(), Error: err, Walker: walker.Identity().PersonaName})
			return err
		}

		emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventTransition, Node: node.Name(), Edge: matchedEdge.ID(), Walker: walker.Identity().PersonaName})

		if matchedEdge.IsLoop() {
			state.IncrementLoop(node.Name())
		}

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		fromZone := zoneForNode(node.Name(), g.zones)
		toZone := zoneForNode(matched.NextNode, g.zones)
		if fromZone != nil && (toZone == nil || fromZone.Name != toZone.Name) {
			applyContextFilter(state.Context, fromZone.ContextFilter)
		}

		if matched.NextNode == g.doneNode {
			state.Status = walkStatusDone
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkComplete, Walker: walker.Identity().PersonaName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = walkStatusError
			err := fmt.Errorf("%w: transition target %q from edge %s", circuit.ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, &circuit.WalkEvent{Type: circuit.EventWalkError, Error: err})
			return err
		}

		priorArtifact = artifact
		priorWalker = walker
		node = nextNode
		steps++
	}
}

// applyContextFilter strips or keeps context keys based on a zone's filter.
// Block takes precedence: if both pass and block are set, blocked keys are
// removed first, then only passed keys survive.
func applyContextFilter(ctx map[string]any, filter *circuit.ContextFilterDef) {
	if filter == nil {
		return
	}
	if len(filter.Block) > 0 {
		for _, key := range filter.Block {
			delete(ctx, key)
		}
	}
	if len(filter.Pass) > 0 {
		allowed := make(map[string]bool, len(filter.Pass))
		for _, key := range filter.Pass {
			allowed[key] = true
		}
		for key := range ctx {
			if !allowed[key] {
				delete(ctx, key)
			}
		}
	}
}

// composeObservers returns a single observer from two possibly-nil observers.
func composeObservers(a, b circuit.WalkObserver) circuit.WalkObserver {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return circuit.MultiObserver{a, b}
}

// nodeCtx returns a derived context with the node's timeout applied.
// If the node has no timeout, returns the parent context and a no-op cancel.
func (g *DefaultGraph) nodeCtx(parent context.Context, nodeName string) (context.Context, context.CancelFunc) {
	if d, ok := g.nodeTimeouts[nodeName]; ok && d > 0 {
		return context.WithTimeout(parent, d)
	}
	return parent, func() {}
}

// walkDelegate handles a DelegateNode encounter during Walk or WalkTeam.
// It calls GenerateCircuit, builds the sub-graph via Runner (which provides
// schema validation and hooks), walks it, and returns a DelegateArtifact
// wrapping the inner walk's results.
func (g *DefaultGraph) walkDelegate(ctx context.Context, walker circuit.Walker, obs circuit.WalkObserver, dn DelegateNode, nc circuit.NodeContext) (*DelegateArtifact, error) {
	circuitType := delegateCircuitType(dn)

	emitEvent(obs, &circuit.WalkEvent{
		Type:   circuit.EventDelegateStart,
		Node:   dn.Name(),
		Walker: walker.Identity().PersonaName,
		Metadata: map[string]any{
			circuit.ExtraKeyCircuitType: circuitType,
		},
	})
	slog.DebugContext(ctx, circuit.LogDelegateStart, circuit.LogKeyComponent, circuit.LogComponentWalk, circuit.LogKeyNode, dn.Name(), circuit.LogKeyCircuit, circuitType)

	circuitDef, err := dn.GenerateCircuit(ctx, nc)
	if err != nil {
		return nil, fmt.Errorf("delegate %s: generate circuit: %w", dn.Name(), err)
	}

	// Update circuit type from the generated def if the node didn't provide it.
	if circuitType == "" && circuitDef != nil {
		circuitType = circuitDef.Circuit
	}

	reg := g.registries
	if reg == nil {
		reg = &GraphRegistries{}
	}

	runner, err := NewRunnerWith(circuitDef, reg)
	if err != nil {
		return nil, fmt.Errorf("delegate %s: build runner: %w", dn.Name(), err)
	}

	subWalker := circuit.NewProcessWalker(walker.State().ID + ":delegate:" + dn.Name())
	id := walker.Identity()
	subWalker.SetIdentity(&id)

	for k, v := range walker.State().Context {
		subWalker.State().Context[k] = v
	}

	prefixObs := &delegateObserver{inner: obs, prefix: "delegate:" + dn.Name() + ":"}
	if dg, ok := runner.Graph.(*DefaultGraph); ok {
		dg.SetObserver(prefixObs)
	}

	start := time.Now()
	walkErr := runner.Walk(ctx, subWalker, string(circuitDef.Start))
	elapsed := time.Since(start)

	da := &DelegateArtifact{
		GeneratedCircuit: circuitDef,
		InnerArtifacts:   subWalker.State().Outputs,
		NodeCount:        len(circuitDef.Nodes),
		Elapsed:          elapsed,
		InnerError:       walkErr,
	}

	outerState := walker.State()
	if outerState.Outputs == nil {
		outerState.Outputs = make(map[string]circuit.Artifact)
	}
	for innerName, art := range subWalker.State().Outputs {
		outerState.Outputs["delegate:"+dn.Name()+":"+innerName] = art
	}

	emitEvent(obs, &circuit.WalkEvent{
		Type:     circuit.EventDelegateEnd,
		Node:     dn.Name(),
		Walker:   walker.Identity().PersonaName,
		Elapsed:  elapsed,
		Artifact: da,
		Error:    walkErr,
		Metadata: map[string]any{
			circuit.ExtraKeyCircuitType: circuitType,
			"node_count":                da.NodeCount,
			"inner_error":               walkErr != nil,
		},
	})

	if walkErr != nil {
		return da, fmt.Errorf("delegate %s: sub-walk: %w", dn.Name(), walkErr)
	}
	return da, nil
}

// delegateCircuitType extracts the target circuit name from a DelegateNode
// when available. For circuitRefNode the name is known statically; for
// dslDelegateNode it is only determined after GenerateCircuit runs, so
// this function returns "" in that case (the caller backfills after generation).
func delegateCircuitType(dn DelegateNode) string {
	if n, ok := dn.(*circuitRefNode); ok && n.circuitDef != nil {
		return n.circuitDef.Circuit
	}
	return ""
}

// delegateObserver wraps a WalkObserver and prefixes all node/edge names
// so outer observers can distinguish inner walk events from outer events.
type delegateObserver struct {
	inner  circuit.WalkObserver
	prefix string
}

func (d *delegateObserver) OnEvent(e *circuit.WalkEvent) {
	if d.inner == nil {
		return
	}
	prefixed := *e
	if prefixed.Node != "" {
		prefixed.Node = d.prefix + prefixed.Node
	}
	if prefixed.Edge != "" {
		prefixed.Edge = d.prefix + prefixed.Edge
	}
	d.inner.OnEvent(&prefixed)
}

// evidenceSNR computes signal-to-noise ratio: outputItems / inputItems.
// Returns 0 when inputItems <= 0 (no signal to measure).
func evidenceSNR(inputItems, outputItems int) float64 {
	if inputItems <= 0 {
		return 0
	}
	return float64(outputItems) / float64(inputItems)
}

// emitEvent is a helper to safely emit an event to a possibly-nil observer.
func emitEvent(obs circuit.WalkObserver, e *circuit.WalkEvent) {
	if obs != nil {
		obs.OnEvent(e)
	}
}

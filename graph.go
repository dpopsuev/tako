package framework

// Category: Core Primitives

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// walkInterrupted is the sentinel error returned by Walk when a node
// signals an Interrupt. The Run() function checks for this to decide
// whether to clean up the checkpoint.
var walkInterrupted = fmt.Errorf("walk interrupted")

// Graph is a directed graph of Nodes connected by Edges, partitioned into Zones.
type Graph interface {
	Name() string
	Nodes() []Node
	Edges() []Edge
	Zones() []Zone
	NodeByName(name string) (Node, bool)
	EdgesFrom(nodeName string) []Edge
	Walk(ctx context.Context, walker Walker, startNode string) error
	WalkTeam(ctx context.Context, team *Team, startNode string) error
}

// Zone is a meta-phase grouping of Nodes with shared characteristics.
type Zone struct {
	Name            string
	NodeNames       []string
	ElementAffinity Element
	Stickiness      int // 0-3 stickiness value for agents in this zone
	Domain          string
	ContextFilter   *ContextFilterDef
}

// DefaultGraph is the reference Graph implementation. It stores nodes and
// edges in maps for O(1) lookup while preserving edge definition order
// for deterministic first-match evaluation.
type DefaultGraph struct {
	name         string
	nodes        []Node
	edges        []Edge
	zones        []Zone
	nodeIndex    map[string]Node
	edgeIndex    map[string][]Edge     // from-node -> edges in definition order
	nodeTimeouts map[string]time.Duration // per-node timeout (from DSL)
	doneNode     string                // terminal pseudo-node name (walk stops here)
	observer     WalkObserver          // graph-level observer, used by Walk and composed with team observer in WalkTeam
	registries   *GraphRegistries      // retained for DelegateNode sub-walk building
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
func WithObserver(obs WalkObserver) GraphOption {
	return func(g *DefaultGraph) {
		g.observer = obs
	}
}

// WithNodeTimeouts sets per-node timeout durations. When a node with a timeout
// is encountered during Walk, a derived context.WithTimeout is created so the
// node's Process (or delegate sub-walk) is cancelled if it exceeds the budget.
func WithNodeTimeouts(m map[string]time.Duration) GraphOption {
	return func(g *DefaultGraph) {
		g.nodeTimeouts = m
	}
}

// SetObserver replaces the graph-level observer. This is useful for consumers
// that build graphs via NewRunnerWith and need to attach observers externally.
func (g *DefaultGraph) SetObserver(obs WalkObserver) {
	g.observer = obs
}

// NewGraph constructs a DefaultGraph from the provided nodes, edges, and zones.
// Returns an error if referential integrity checks fail (e.g. an edge
// references a nonexistent node).
func NewGraph(name string, nodes []Node, edges []Edge, zones []Zone, opts ...GraphOption) (*DefaultGraph, error) {
	g := &DefaultGraph{
		name:      name,
		nodes:     nodes,
		edges:     edges,
		zones:     zones,
		nodeIndex: make(map[string]Node, len(nodes)),
		edgeIndex: make(map[string][]Edge),
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
			return nil, fmt.Errorf("%w: edge %s references source %q", ErrNodeNotFound, e.ID(), e.From())
		}
		to := e.To()
		if to != g.doneNode {
			if _, ok := g.nodeIndex[to]; !ok {
				return nil, fmt.Errorf("%w: edge %s references target %q", ErrNodeNotFound, e.ID(), to)
			}
		}
		g.edgeIndex[e.From()] = append(g.edgeIndex[e.From()], e)
	}

	return g, nil
}

func (g *DefaultGraph) Name() string    { return g.name }
func (g *DefaultGraph) Nodes() []Node   { return g.nodes }
func (g *DefaultGraph) Edges() []Edge   { return g.edges }
func (g *DefaultGraph) Zones() []Zone   { return g.zones }

func (g *DefaultGraph) NodeByName(name string) (Node, bool) {
	n, ok := g.nodeIndex[name]
	return n, ok
}

func (g *DefaultGraph) EdgesFrom(nodeName string) []Edge {
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
func (g *DefaultGraph) Walk(ctx context.Context, walker Walker, startNode string) error {
	obs := g.observer
	walkerName := walker.Identity().PersonaName

	node, ok := g.nodeIndex[startNode]
	if !ok {
		err := fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)
		emitEvent(obs, WalkEvent{Type: EventWalkError, Node: startNode, Error: err})
		return err
	}

	state := walker.State()
	state.CurrentNode = startNode
	var priorArtifact Artifact

	for {
		if err := ctx.Err(); err != nil {
			state.Status = "error"
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventNodeEnter, Node: node.Name(), Walker: walkerName})
		slog.Debug(LogNodeEnter, LogKeyComponent, LogComponentWalk, "node", node.Name(), "walker", walkerName)
		nodeStart := time.Now()

		nc := NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		nodeCtx, nodeCancel := g.nodeCtx(ctx, node.Name())

		var artifact Artifact
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
				state.Status = "interrupted"
				if intr.Data != nil {
					state.Context["interrupt_data"] = intr.Data
				}
				emitEvent(obs, WalkEvent{
					Type:   EventWalkInterrupted,
					Node:   node.Name(),
					Walker: walkerName,
					Metadata: map[string]any{
						"reason": intr.Reason,
					},
				})
				return walkInterrupted
			}
			state.Status = "error"
			emitEvent(obs, WalkEvent{Type: EventNodeExit, Node: node.Name(), Walker: walkerName, Elapsed: nodeElapsed, Error: err})
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		exitMeta := map[string]any{}
		if ca, ok := artifact.(CountableArtifact); ok {
			exitMeta["snr"] = evidenceSNR(ca.InputCount(), ca.OutputCount())
		}
		emitEvent(obs, WalkEvent{Type: EventNodeExit, Node: node.Name(), Walker: walkerName, Artifact: artifact, Elapsed: nodeElapsed, Metadata: exitMeta})
		slog.Debug(LogNodeExit, LogKeyComponent, LogComponentWalk, "node", node.Name(), "elapsed_ms", nodeElapsed.Milliseconds())

		if artifact != nil && artifact.Confidence() > 0 {
			state.RecordConfidence(artifact.Confidence())
		}

		if state.Outputs == nil {
			state.Outputs = make(map[string]Artifact)
		}
		state.Outputs[node.Name()] = artifact

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Node: node.Name(), Walker: walkerName})
			return nil
		}

		// Evaluate all edges, separating parallel from sequential matches.
		// If 2+ parallel edges match, fan-out to concurrent execution.
		var parallelMatches []parallelMatch
		var seqMatch *Transition
		var seqEdge Edge
		for _, e := range edges {
			emitEvent(obs, WalkEvent{Type: EventEdgeEvaluate, Node: node.Name(), Edge: e.ID()})
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
			mergeNodeName, err := g.walkFanOut(ctx, walker, obs, node, artifact, parallelMatches)
			if err != nil {
				return err
			}
			if mergeNodeName == g.doneNode {
				state.Status = "done"
				emitEvent(obs, WalkEvent{Type: EventWalkComplete, Walker: walkerName})
				return nil
			}
			nextNode, ok := g.nodeIndex[mergeNodeName]
			if !ok {
				state.Status = "error"
				err := fmt.Errorf("%w: merge target %q", ErrNodeNotFound, mergeNodeName)
				emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
				return err
			}
			priorArtifact = artifact
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
			state.Status = "error"
			err := fmt.Errorf("%w: node %q, artifact type %q", ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventTransition, Node: node.Name(), Edge: matchedEdge.ID()})
		slog.Debug(LogEdgeTaken, LogKeyComponent, LogComponentWalk, LogKeyFrom, node.Name(), LogKeyEdge, matchedEdge.ID(), LogKeyTo, matched.NextNode, LogKeyLoop, matchedEdge.IsLoop(), LogKeyShortcut, matchedEdge.IsShortcut())

		if matchedEdge.IsLoop() {
			state.IncrementLoop(node.Name())
			slog.Debug(LogLoopIncremented, LogKeyComponent, LogComponentWalk, LogKeyNode, node.Name(), LogKeyCount, state.LoopCounts[node.Name()])
		}

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		fromZone := zoneForNode(node.Name(), g.zones)
		toZone := zoneForNode(matched.NextNode, g.zones)
		if fromZone != nil && (toZone == nil || fromZone.Name != toZone.Name) {
			applyContextFilter(state.Context, fromZone.ContextFilter)
		}

		if matched.NextNode == g.doneNode {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Walker: walkerName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = "error"
			err := fmt.Errorf("%w: transition target %q from edge %s", ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
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
func (g *DefaultGraph) WalkTeam(ctx context.Context, team *Team, startNode string) error {
	obs := composeObservers(g.observer, team.Observer)

	node, ok := g.nodeIndex[startNode]
	if !ok {
		emitEvent(obs, WalkEvent{Type: EventWalkError, Node: startNode, Error: fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)})
		return fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)
	}

	if len(team.Walkers) == 0 {
		return fmt.Errorf("team has no walkers")
	}

	var priorWalker Walker
	var priorArtifact Artifact
	steps := 0

	for {
		if err := ctx.Err(); err != nil {
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		if team.MaxSteps > 0 && steps >= team.MaxSteps {
			err := fmt.Errorf("max steps (%d) exceeded at node %q", team.MaxSteps, node.Name())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
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
			emitEvent(obs, WalkEvent{
				Type:     EventWalkerSwitch,
				Node:     node.Name(),
				Walker:   walker.Identity().PersonaName,
				Metadata: meta,
			})
		}

		emitEvent(obs, WalkEvent{Type: EventNodeEnter, Node: node.Name(), Walker: walker.Identity().PersonaName})
		nodeStart := time.Now()

		state := walker.State()
		state.CurrentNode = node.Name()

		nc := NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		nodeCtx, nodeCancel := g.nodeCtx(ctx, node.Name())

		var artifact Artifact
		var err error
		if dn, isDel := node.(DelegateNode); isDel {
			artifact, err = g.walkDelegate(nodeCtx, walker, obs, dn, nc)
		} else {
			artifact, err = walker.Handle(nodeCtx, node, nc)
		}
		nodeCancel()
		nodeElapsed := time.Since(nodeStart)

		if err != nil {
			state.Status = "error"
			emitEvent(obs, WalkEvent{
				Type:    EventNodeExit,
				Node:    node.Name(),
				Walker:  walker.Identity().PersonaName,
				Elapsed: nodeElapsed,
				Error:   err,
			})
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		teamExitMeta := map[string]any{}
		if ca, ok := artifact.(CountableArtifact); ok {
			teamExitMeta["snr"] = evidenceSNR(ca.InputCount(), ca.OutputCount())
		}
		emitEvent(obs, WalkEvent{
			Type:     EventNodeExit,
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
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Node: node.Name(), Walker: walker.Identity().PersonaName})
			return nil
		}

		var matched *Transition
		var matchedEdge Edge
		for _, e := range edges {
			emitEvent(obs, WalkEvent{Type: EventEdgeEvaluate, Node: node.Name(), Edge: e.ID()})
			t := e.Evaluate(artifact, state)
			if t != nil {
				matched = t
				matchedEdge = e
				break
			}
		}

		if matched == nil {
			state.Status = "error"
			err := fmt.Errorf("%w: node %q, artifact type %q", ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventTransition, Node: node.Name(), Edge: matchedEdge.ID()})

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
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Walker: walker.Identity().PersonaName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = "error"
			err := fmt.Errorf("%w: transition target %q from edge %s", ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
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
func applyContextFilter(ctx map[string]any, filter *ContextFilterDef) map[string]any {
	if filter == nil {
		return ctx
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
	return ctx
}

// composeObservers returns a single observer from two possibly-nil observers.
func composeObservers(a, b WalkObserver) WalkObserver {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return MultiObserver{a, b}
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
func (g *DefaultGraph) walkDelegate(ctx context.Context, walker Walker, obs WalkObserver, dn DelegateNode, nc NodeContext) (*DelegateArtifact, error) {
	circuitType := delegateCircuitType(dn)

	emitEvent(obs, WalkEvent{
		Type:   EventDelegateStart,
		Node:   dn.Name(),
		Walker: walker.Identity().PersonaName,
		Metadata: map[string]any{
			"circuit_type": circuitType,
		},
	})
	slog.Debug(LogDelegateStart, LogKeyComponent, LogComponentWalk, LogKeyNode, dn.Name(), LogKeyCircuit, circuitType)

	circuitDef, err := dn.GenerateCircuit(ctx, nc)
	if err != nil {
		return nil, fmt.Errorf("delegate %s: generate circuit: %w", dn.Name(), err)
	}

	// Update circuit type from the generated def if the node didn't provide it.
	if circuitType == "" && circuitDef != nil {
		circuitType = circuitDef.Circuit
	}

	var reg GraphRegistries
	if g.registries != nil {
		reg = *g.registries
	}

	runner, err := NewRunnerWith(circuitDef, reg)
	if err != nil {
		return nil, fmt.Errorf("delegate %s: build runner: %w", dn.Name(), err)
	}

	subWalker := NewProcessWalker(walker.State().ID + ":delegate:" + dn.Name())
	subWalker.SetIdentity(walker.Identity())

	for k, v := range walker.State().Context {
		subWalker.State().Context[k] = v
	}

	prefixObs := &delegateObserver{inner: obs, prefix: "delegate:" + dn.Name() + ":"}
	if dg, ok := runner.Graph.(*DefaultGraph); ok {
		dg.SetObserver(prefixObs)
	}

	start := time.Now()
	walkErr := runner.Walk(ctx, subWalker, circuitDef.Start)
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
		outerState.Outputs = make(map[string]Artifact)
	}
	for innerName, art := range subWalker.State().Outputs {
		outerState.Outputs["delegate:"+dn.Name()+":"+innerName] = art
	}

	emitEvent(obs, WalkEvent{
		Type:     EventDelegateEnd,
		Node:     dn.Name(),
		Walker:   walker.Identity().PersonaName,
		Elapsed:  elapsed,
		Artifact: da,
		Error:    walkErr,
		Metadata: map[string]any{
			"circuit_type": circuitType,
			"node_count":   da.NodeCount,
			"inner_error":  walkErr != nil,
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
	switch n := dn.(type) {
	case *circuitRefNode:
		if n.circuitDef != nil {
			return n.circuitDef.Circuit
		}
	}
	return ""
}

// delegateObserver wraps a WalkObserver and prefixes all node/edge names
// so outer observers can distinguish inner walk events from outer events.
type delegateObserver struct {
	inner  WalkObserver
	prefix string
}

func (d *delegateObserver) OnEvent(e WalkEvent) {
	if d.inner == nil {
		return
	}
	if e.Node != "" {
		e.Node = d.prefix + e.Node
	}
	if e.Edge != "" {
		e.Edge = d.prefix + e.Edge
	}
	d.inner.OnEvent(e)
}

// evidenceSNR computes signal-to-noise ratio: outputItems / inputItems.
// Returns 0 when inputItems <= 0 (no signal to measure).
func evidenceSNR(inputItems, outputItems int) float64 {
	if inputItems <= 0 {
		return 0
	}
	return float64(outputItems) / float64(inputItems)
}

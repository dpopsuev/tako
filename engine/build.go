package engine

// Category: DSL & Build — graph construction and registries.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/troupe/identity"
)

// Type aliases — definitions live in circuit/ sub-package.

// Forwarded functions from circuit/.
var (
	LoadCircuit      = circuit.LoadCircuit
	ValidateArtifact = circuit.ValidateArtifact
	ResolveInput     = circuit.ResolveInput
	RenderPrompt     = circuit.RenderPrompt
	MergeVars        = circuit.MergeVars
)

// Inproc instrument names — in-process Go handlers exposed as instruments.
// Nodes declare these via instrument: field (e.g. instrument: transformer).
const (
	InstrumentTransformer = "transformer"
	InstrumentExtractor   = "extractor"
	InstrumentRenderer    = "renderer"
	InstrumentNode        = "node"
	InstrumentDelegate    = "delegate"
	InstrumentCircuit     = "circuit"
)

// Merge strategy constants forwarded from circuit/.
const (
	MergeAppend = circuit.MergeAppend
	MergeLatest = circuit.MergeLatest
)

// NodeRegistry maps node family names to Node factory functions.
type NodeRegistry map[string]func(def circuit.NodeDef) circuit.Node

// EdgeFactory maps edge IDs to Edge factory functions.
type EdgeFactory map[string]func(def circuit.EdgeDef) circuit.Edge

// ComponentLoader resolves an import name to a live Component.
type ComponentLoader func(name string) (*Component, error)

// GraphRegistries bundles all optional registries for BuildGraph.
type GraphRegistries struct {
	Nodes            NodeRegistry
	Edges            EdgeFactory
	Extractors       ExtractorRegistry
	Renderers        RendererRegistry
	Instruments      InstrumentRegistry
	Hooks            HookRegistry
	Components       ComponentLoader
	Circuits         map[string]*circuit.CircuitDef
	Manifests        ManifestRegistry
	InstrumentDir    string // working directory for instrument commands
	MediatorEndpoint string

	// Approval gate wiring.
	ApprovalStore    gate.ApprovalStore // optional — parks output at gated nodes
	ApprovalNotifier gate.Notifier      // optional — sends notifications when items are parked
}

// BuildGraph constructs a Graph from a circuit.CircuitDef using the full registries bundle.
//
//nolint:gocyclo // graph construction from full definition — sequential setup steps
func BuildGraph(def *circuit.CircuitDef, reg *GraphRegistries) (Graph, error) {
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	if len(def.Imports) > 0 && reg.Components != nil {
		comps := make([]*Component, 0, len(def.Imports))
		for _, imp := range def.Imports {
			c, err := reg.Components(imp)
			if err != nil {
				return nil, fmt.Errorf("import %q: %w", imp, err)
			}
			comps = append(comps, c)
		}
		merged, err := MergeComponents(reg, comps...)
		if err != nil {
			return nil, fmt.Errorf("merge imports: %w", err)
		}
		reg.Instruments = merged.Instruments
		reg.Extractors = merged.Extractors
		reg.Hooks = merged.Hooks
	}

	fwNodes := make([]circuit.Node, 0, len(def.Nodes))
	for i := range def.Nodes {
		node, err := resolveNode(def, &def.Nodes[i], reg)
		if err != nil {
			return nil, err
		}
		fwNodes = append(fwNodes, node)
	}

	fwEdges := make([]circuit.Edge, 0, len(def.Edges))
	for i := range def.Edges {
		ed := &def.Edges[i]
		switch {
		case ed.When != "":
			exprEdge, err := CompileExpressionEdge(&def.Edges[i], def.Vars)
			if err != nil {
				return nil, fmt.Errorf("edge %s: %w", ed.ID, err)
			}
			fwEdges = append(fwEdges, exprEdge)
		case reg.Edges != nil:
			if factory, ok := reg.Edges[ed.ID]; ok {
				fwEdges = append(fwEdges, factory(def.Edges[i]))
			} else {
				fwEdges = append(fwEdges, &dslEdge{def: def.Edges[i]})
			}
		default:
			fwEdges = append(fwEdges, &dslEdge{def: def.Edges[i]})
		}
	}

	fwZones := make([]Zone, 0, len(def.Zones))
	for name, zd := range def.Zones {
		elem, _ := identity.ResolveApproach(strings.ToLower(zd.Approach))
		nodeNames := make([]string, len(zd.Nodes))
		for j, nn := range zd.Nodes {
			nodeNames[j] = string(nn)
		}
		fwZones = append(fwZones, Zone{
			Name:            name,
			NodeNames:       nodeNames,
			Approach: elem,
			Stickiness:      zd.Stickiness,
			Domain:          strings.ToLower(zd.Domain),
			ContextFilter:   zd.ContextFilter,
		})
	}

	var timeouts map[string]time.Duration
	for i := range def.Nodes {
		d, err := def.Nodes[i].EffectiveTimeout(def.Timeout)
		if err != nil {
			return nil, err
		}
		if d > 0 {
			if timeouts == nil {
				timeouts = make(map[string]time.Duration)
			}
			timeouts[string(def.Nodes[i].Name)] = d
		}
	}

	var gatedNodes map[string]string
	for i := range def.Nodes {
		if def.Nodes[i].Gate != "" {
			if gatedNodes == nil {
				gatedNodes = make(map[string]string)
			}
			gatedNodes[string(def.Nodes[i].Name)] = def.Nodes[i].Gate
		}
	}

	opts := []GraphOption{WithDoneNode(string(def.Done))}
	if def.Finally != "" {
		opts = append(opts, WithFinallyNode(string(def.Finally)))
	}
	if len(timeouts) > 0 {
		opts = append(opts, WithNodeTimeouts(timeouts))
	}
	if len(gatedNodes) > 0 {
		opts = append(opts, WithGatedNodes(gatedNodes))
	}
	if reg.ApprovalStore != nil {
		opts = append(opts, WithApprovalStore(reg.ApprovalStore))
	}
	if reg.ApprovalNotifier != nil {
		opts = append(opts, WithApprovalNotifier(reg.ApprovalNotifier))
	}

	opts = append(opts, WithRegistries(reg))
	g, err := NewGraph(def.Circuit, fwNodes, fwEdges, fwZones, opts...)
	if err != nil {
		return nil, err
	}

	if def.Topology != "" {
		if err := validateTopology(g, def); err != nil {
			return nil, err
		}
	}

	runBuildDiagnostics(def, reg)

	return g, nil
}

// validateTopology checks the graph against the declared topology.
func validateTopology(g *DefaultGraph, def *circuit.CircuitDef) error {
	v := circuit.DefaultTopologyValidator
	if v == nil {
		slog.WarnContext(context.Background(), circuit.LogTopologySkipped, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyTopology, def.Topology), slog.Any(circuit.LogKeyCircuit, def.Circuit))
		return nil
	}
	shape := buildGraphShape(g, def)
	return v(def.Topology, shape)
}

func buildGraphShape(g *DefaultGraph, def *circuit.CircuitDef) circuit.GraphShape {
	nodes := make([]circuit.GraphNodeInfo, 0, len(g.nodes))
	for _, n := range g.nodes {
		inputs := 0
		for _, e := range g.edges {
			if e.To() == n.Name() && !e.IsShortcut() && !e.IsLoop() {
				inputs++
			}
		}
		outputs := 0
		for _, e := range g.edges {
			if e.From() == n.Name() && !e.IsShortcut() && !e.IsLoop() {
				outputs++
			}
		}
		nodes = append(nodes, circuit.GraphNodeInfo{
			Name:    n.Name(),
			Inputs:  inputs,
			Outputs: outputs,
		})
	}
	return circuit.GraphShape{
		StartNode: string(def.Start),
		DoneNode:  g.doneNode,
		Nodes:     nodes,
	}
}

// resolveNode creates a Node from a circuit.NodeDef using instrument + action.
func resolveNode(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries) (circuit.Node, error) {
	elem, _ := identity.ResolveApproach(strings.ToLower(nd.Approach))
	return resolveByInstrument(def, nd, reg, elem)
}

// InprocResolver resolves a node definition into a concrete circuit.Node
// for dispatch: inproc instruments (in-process Go handlers).
type InprocResolver func(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error)

// inprocResolvers maps inproc instrument names to their Go handler resolvers.
// These are the built-in in-process instruments: transformer, extractor, etc.
var inprocResolvers = map[string]InprocResolver{
	InstrumentTransformer: resolveTransformerHandler,
	InstrumentExtractor:   resolveExtractorHandler,
	InstrumentRenderer:    resolveRendererHandler,
	InstrumentNode:        resolveNodeHandler,
	InstrumentDelegate:    resolveDelegateHandler,
	InstrumentCircuit:     resolveCircuitHandler,
}

func resolveByInstrument(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	instrument := nd.Instrument
	if instrument == "" {
		return nil, fmt.Errorf("%w: %q: instrument is required", ErrNode, name)
	}

	// Manifest-based dispatch — cli, mcp, container instruments.
	if reg.Manifests != nil {
		if manifest, ok := reg.Manifests[instrument]; ok {
			return resolveInstrumentNode(def, nd, manifest, elem, reg.InstrumentDir)
		}
	}

	// Inproc dispatch — built-in Go handlers (transformer, extractor, etc.).
	resolver, ok := inprocResolvers[instrument]
	if !ok {
		return nil, fmt.Errorf("%w: %q: unknown instrument %q", ErrNode, name, instrument)
	}
	return resolver(def, nd, reg, elem)
}

func resolveTransformerHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	t, err := resolveTransformerByName(def, action, name, reg)
	if err != nil {
		return nil, err
	}
	return &transformerNode{
		baseNode:   baseNode{name: name, element: elem},
		trans:      t,
		prompt:     nd.Prompt,
		input:      nd.Input,
		provider:   nd.Provider,
		config:     def.Vars,
		nodeConfig: nd.EffectiveConfig(),
	}, nil
}

func resolveExtractorHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	ext, err := resolveExtractor(def, action, nd, reg)
	if err != nil {
		return nil, err
	}
	return &extractorNode{
		baseNode: baseNode{name: name, element: elem},
		ext:      ext,
	}, nil
}

func resolveRendererHandler(_ *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	rnd, err := resolveRenderer(action, nd, reg)
	if err != nil {
		return nil, err
	}
	return &rendererNode{
		baseNode: baseNode{name: name, element: elem},
		rnd:      rnd,
	}, nil
}

func resolveNodeHandler(_ *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	if reg.Nodes == nil {
		return nil, fmt.Errorf("%w: %q: action %q not found (node registry is nil)", ErrNode, name, action)
	}
	factory, ok := reg.Nodes[action]
	if !ok {
		return nil, fmt.Errorf("%w: %q: action %q not found in node registry", ErrNode, name, action)
	}
	return factory(*nd), nil
}

func resolveDelegateHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	if reg.Instruments == nil {
		return nil, fmt.Errorf("%w: %q: delegate action %q not found (instrument registry is nil)", ErrNode, name, action)
	}
	gen, err := reg.Instruments.Get(action)
	if err != nil {
		return nil, fmt.Errorf("node %q: delegate action: %w", name, err)
	}
	return &dslDelegateNode{
		baseNode:   baseNode{name: name, element: elem},
		gen:        gen,
		config:     def.Vars,
		nodeConfig: nd.EffectiveConfig(),
	}, nil
}

func resolveCircuitHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem identity.Element) (circuit.Node, error) {
	name := string(nd.Name)
	action := nd.Action
	if action == "" {
		action = name
	}
	switch {
	case reg.Circuits != nil && reg.Circuits[action] != nil:
		cd := reg.Circuits[action]
		slog.DebugContext(context.Background(), circuit.LogCircuitHandlerLocal, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyNode, name), slog.Any(circuit.LogKeyHandler, action))
		return &circuitRefNode{
			baseNode:   baseNode{name: name, element: elem},
			circuitDef: cd,
		}, nil
	case reg.MediatorEndpoint != "":
		slog.DebugContext(context.Background(), circuit.LogCircuitHandlerMediator, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyNode, name), slog.Any(circuit.LogKeyHandler, action), slog.Any(circuit.LogKeyEndpoint, reg.MediatorEndpoint))
		return &transformerNode{
			baseNode:   baseNode{name: name, element: elem},
			trans:      &MCPCircuitTransformer{CircuitType: action, Endpoint: reg.MediatorEndpoint},
			config:     def.Vars,
			nodeConfig: nd.EffectiveConfig(),
		}, nil
	default:
		return nil, fmt.Errorf("%w: %q: circuit action %q not found (no local circuit and no mediator endpoint)", ErrNode, name, action)
	}
}

// resolveTransformerByName resolves an instrument by name, checking builtins first.
func resolveTransformerByName(_ *circuit.CircuitDef, name, nodeName string, reg *GraphRegistries) (Instrument, error) {
	switch name {
	case BuiltinTransformerGoTemplate:
		return &goTemplateTransformer{}, nil
	case BuiltinTransformerPassthrough:
		return &passthroughTransformer{}, nil
	}
	if reg.Instruments == nil {
		return nil, fmt.Errorf("%w: %q: instrument %q not found (registry is nil)", ErrNode, nodeName, name)
	}
	return reg.Instruments.Get(name)
}

// dslEdge is a default Edge implementation created from an circuit.EdgeDef when
// no custom factory is registered. It always matches (returns a transition).
type dslEdge struct {
	def circuit.EdgeDef
}

func (e *dslEdge) ID() string            { return e.def.ID }
func (e *dslEdge) From() string          { return string(e.def.From) }
func (e *dslEdge) To() string            { return string(e.def.To) }
func (e *dslEdge) IsShortcut() bool      { return e.def.Shortcut }
func (e *dslEdge) IsLoop() bool          { return e.def.Loop }
func (e *dslEdge) IsParallel() bool      { return e.def.Parallel }
func (e *dslEdge) MergeStrategy() string { return e.def.Merge }
func (e *dslEdge) Evaluate(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
	return &circuit.Transition{
		NextNode:    string(e.def.To),
		Explanation: e.def.Condition,
	}
}

// resolveExtractor resolves an extractor by name.
func resolveExtractor(def *circuit.CircuitDef, name string, nd *circuit.NodeDef, reg *GraphRegistries) (Extractor, error) {
	nodeName := string(nd.Name)
	switch name {
	case BuiltinExtractorJSONSchema:
		return &JSONSchemaExtractor{Schema: nd.Schema}, nil
	case BuiltinExtractorRegex:
		cfg := nd.EffectiveConfig()
		pattern := cfg.Pattern
		if pattern == "" {
			return nil, fmt.Errorf("%w: %q: regex extractor requires meta.pattern", ErrNode, nodeName)
		}
		return NewRegexExtractor(nodeName, pattern)
	}

	for _, ed := range def.Extractors {
		if ed.Name != name {
			continue
		}
		switch ed.Type {
		case BuiltinExtractorJSONSchema:
			schema := ed.Schema
			if nd.Schema != nil {
				schema = nd.Schema
			}
			return &JSONSchemaExtractor{Schema: schema}, nil
		case BuiltinExtractorRegex:
			if ed.Pattern == "" {
				return nil, fmt.Errorf("%w: %q: regex type requires pattern", ErrExtractor, ed.Name)
			}
			return NewRegexExtractor(ed.Name, ed.Pattern)
		default:
			return nil, fmt.Errorf("%w: %q: unknown type %q", ErrExtractor, ed.Name, ed.Type)
		}
	}

	if reg.Extractors != nil {
		ext, err := reg.Extractors.Get(name)
		if err == nil {
			return ext, nil
		}
	}
	return nil, fmt.Errorf("%w: %q: extractor %q not found", ErrNode, string(nd.Name), name)
}

// resolveRenderer resolves a renderer by name.
func resolveRenderer(name string, nd *circuit.NodeDef, reg *GraphRegistries) (Renderer, error) {
	if name == BuiltinRendererTemplate {
		return &TemplateRenderer{Template: nd.Prompt}, nil
	}
	if reg.Renderers != nil {
		rnd, err := reg.Renderers.Get(name)
		if err == nil {
			return rnd, nil
		}
	}
	return nil, fmt.Errorf("%w: %q: renderer %q not found", ErrNode, string(nd.Name), name)
}

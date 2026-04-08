package engine

// Category: DSL & Build — graph construction and registries.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
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

// Handler type constants forwarded from circuit/.
const (
	HandlerTypeTransformer = circuit.HandlerTypeTransformer
	HandlerTypeExtractor   = circuit.HandlerTypeExtractor
	HandlerTypeRenderer    = circuit.HandlerTypeRenderer
	HandlerTypeNode        = circuit.HandlerTypeNode
	HandlerTypeDelegate    = circuit.HandlerTypeDelegate
	HandlerTypeCircuit     = circuit.HandlerTypeCircuit
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
	Transformers     TransformerRegistry
	Hooks            HookRegistry
	Components       ComponentLoader
	Circuits         map[string]*circuit.CircuitDef
	MediatorEndpoint string
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
		reg.Transformers = merged.Transformers
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
		elem, _ := roster.ResolveApproach(strings.ToLower(zd.Approach))
		nodeNames := make([]string, len(zd.Nodes))
		for j, nn := range zd.Nodes {
			nodeNames[j] = string(nn)
		}
		fwZones = append(fwZones, Zone{
			Name:            name,
			NodeNames:       nodeNames,
			ElementAffinity: elem,
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

	opts := []GraphOption{WithDoneNode(string(def.Done))}
	if def.Finally != "" {
		opts = append(opts, WithFinallyNode(string(def.Finally)))
	}
	if len(timeouts) > 0 {
		opts = append(opts, WithNodeTimeouts(timeouts))
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

// resolveNode creates a Node from a circuit.NodeDef using handler + handler_type.
func resolveNode(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries) (circuit.Node, error) {
	elem, _ := roster.ResolveApproach(strings.ToLower(nd.Approach))
	return resolveHandler(def, nd, reg, elem)
}

// resolveHandler resolves a node using the explicit handler + handler_type path.
// HandlerTypeResolver resolves a node definition into a concrete circuit.Node
// based on its handler_type. Registered in the HandlerTypeRegistry.
type HandlerTypeResolver func(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error)

// HandlerTypeRegistry maps handler_type strings to their resolvers.
// Extensible: consumers can register custom handler types.
type HandlerTypeRegistry map[string]HandlerTypeResolver

// builtinHandlerTypes is the default set of handler type resolvers.
var builtinHandlerTypes = HandlerTypeRegistry{
	HandlerTypeTransformer: resolveTransformerHandler,
	HandlerTypeExtractor:   resolveExtractorHandler,
	HandlerTypeRenderer:    resolveRendererHandler,
	HandlerTypeNode:        resolveNodeHandler,
	HandlerTypeDelegate:    resolveDelegateHandler,
	HandlerTypeCircuit:     resolveCircuitHandler,
}

func resolveHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	handler := nd.Handler
	if handler == "" {
		handler = string(nd.Name)
	}
	ht := nd.HandlerType
	if ht == "" {
		ht = def.HandlerType
	}
	name := string(nd.Name)
	if ht == "" {
		return nil, fmt.Errorf("%w: %q: handler %q specified but no handler_type on node or circuit", ErrNode, name, handler)
	}

	resolver, ok := builtinHandlerTypes[ht]
	if !ok {
		return nil, fmt.Errorf("%w: %q: unknown handler_type %q", ErrNode, name, ht)
	}
	return resolver(def, nd, reg, elem)
}

func resolveTransformerHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	t, err := resolveTransformerByName(def, handler, name, reg)
	if err != nil {
		return nil, err
	}
	return &transformerNode{
		name:       name,
		element:    elem,
		trans:      t,
		prompt:     nd.Prompt,
		input:      nd.Input,
		provider:   nd.Provider,
		config:     def.Vars,
		nodeConfig: nd.EffectiveConfig(),
	}, nil
}

func resolveExtractorHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	ext, err := resolveExtractor(def, handler, nd, reg)
	if err != nil {
		return nil, err
	}
	return &extractorNode{
		name:    name,
		element: elem,
		ext:     ext,
	}, nil
}

func resolveRendererHandler(_ *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	rnd, err := resolveRenderer(handler, nd, reg)
	if err != nil {
		return nil, err
	}
	return &rendererNode{
		name:    name,
		element: elem,
		rnd:     rnd,
	}, nil
}

func resolveNodeHandler(_ *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	if reg.Nodes == nil {
		return nil, fmt.Errorf("%w: %q: handler %q not found (node registry is nil)", ErrNode, name, handler)
	}
	factory, ok := reg.Nodes[handler]
	if !ok {
		return nil, fmt.Errorf("%w: %q: handler %q not found in node registry", ErrNode, name, handler)
	}
	return factory(*nd), nil
}

func resolveDelegateHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	if reg.Transformers == nil {
		return nil, fmt.Errorf("%w: %q: delegate handler %q not found (transformer registry is nil)", ErrNode, name, handler)
	}
	gen, err := reg.Transformers.Get(handler)
	if err != nil {
		return nil, fmt.Errorf("node %q: delegate handler: %w", name, err)
	}
	return &dslDelegateNode{
		name:       name,
		element:    elem,
		gen:        gen,
		config:     def.Vars,
		nodeConfig: nd.EffectiveConfig(),
	}, nil
}

func resolveCircuitHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem roster.Element) (circuit.Node, error) {
	name := string(nd.Name)
	handler := nd.Handler
	if handler == "" {
		handler = name
	}
	switch {
	case reg.Circuits != nil && reg.Circuits[handler] != nil:
		cd := reg.Circuits[handler]
		slog.DebugContext(context.Background(), circuit.LogCircuitHandlerLocal, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyNode, name), slog.Any(circuit.LogKeyHandler, handler))
		return &circuitRefNode{
			name:       name,
			element:    elem,
			circuitDef: cd,
		}, nil
	case reg.MediatorEndpoint != "":
		slog.DebugContext(context.Background(), circuit.LogCircuitHandlerMediator, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyNode, name), slog.Any(circuit.LogKeyHandler, handler), slog.Any(circuit.LogKeyEndpoint, reg.MediatorEndpoint))
		return &transformerNode{
			name:       name,
			element:    elem,
			trans:      &MCPCircuitTransformer{CircuitType: handler, Endpoint: reg.MediatorEndpoint},
			config:     def.Vars,
			nodeConfig: nd.EffectiveConfig(),
		}, nil
	default:
		return nil, fmt.Errorf("%w: %q: circuit handler %q not found (no local circuit and no mediator endpoint)", ErrNode, name, handler)
	}
}

// resolveTransformerByName resolves a transformer by name, checking builtins first.
func resolveTransformerByName(_ *circuit.CircuitDef, name, nodeName string, reg *GraphRegistries) (Transformer, error) {
	switch name {
	case BuiltinTransformerGoTemplate:
		return &goTemplateTransformer{}, nil
	case BuiltinTransformerPassthrough:
		return &passthroughTransformer{}, nil
	}
	if reg.Transformers == nil {
		return nil, fmt.Errorf("%w: %q: transformer %q not found (registry is nil)", ErrNode, nodeName, name)
	}
	return reg.Transformers.Get(name)
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

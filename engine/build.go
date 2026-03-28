package engine

// Category: DSL & Build — graph construction and registries.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// Type aliases — definitions live in circuit/ sub-package.

// Forwarded functions from circuit/.
var (
	LoadCircuit     = circuit.LoadCircuit
	ValidateArtifact = circuit.ValidateArtifact
	ResolveInput    = circuit.ResolveInput
	RenderPrompt    = circuit.RenderPrompt
	MergeVars       = circuit.MergeVars
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
		elem, _ := circuit.ResolveApproach(strings.ToLower(zd.Approach))
		fwZones = append(fwZones, Zone{
			Name:            name,
			NodeNames:       zd.Nodes,
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
			timeouts[def.Nodes[i].Name] = d
		}
	}

	opts := []GraphOption{WithDoneNode(def.Done)}
	if len(timeouts) > 0 {
		opts = append(opts, WithNodeTimeouts(timeouts))
	}

	g, err := NewGraph(def.Circuit, fwNodes, fwEdges, fwZones, opts...)
	if err != nil {
		return nil, err
	}
	g.registries = reg

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
		slog.WarnContext(context.Background(), "topology validator not registered, skipping validation",
			"component", "build",
			"topology", def.Topology,
			"circuit", def.Circuit,
		)
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
		StartNode: def.Start,
		DoneNode:  g.doneNode,
		Nodes:     nodes,
	}
}

// resolveNode creates a Node from a circuit.NodeDef using handler + handler_type.
func resolveNode(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries) (circuit.Node, error) {
	elem, _ := circuit.ResolveApproach(strings.ToLower(nd.Approach))
	return resolveHandler(def, nd, reg, elem)
}

// resolveHandler resolves a node using the explicit handler + handler_type path.
func resolveHandler(def *circuit.CircuitDef, nd *circuit.NodeDef, reg *GraphRegistries, elem circuit.Element) (circuit.Node, error) {
	handler := nd.Handler
	if handler == "" {
		handler = nd.Name
	}
	ht := nd.HandlerType
	if ht == "" {
		ht = def.HandlerType
	}
	if ht == "" {
		return nil, fmt.Errorf("node %q: handler %q specified but no handler_type on node or circuit", nd.Name, handler)
	}

	switch ht {
	case HandlerTypeTransformer:
		t, err := resolveTransformerByName(def, handler, nd.Name, reg)
		if err != nil {
			return nil, err
		}
		return &transformerNode{
			name:       nd.Name,
			element:    elem,
			trans:      t,
			prompt:     nd.Prompt,
			input:      nd.Input,
			provider:   nd.Provider,
			config:     def.Vars,
			nodeConfig: nd.EffectiveConfig(),
			meta:       nd.Meta,
		}, nil

	case HandlerTypeExtractor:
		ext, err := resolveExtractor(def, handler, nd, reg)
		if err != nil {
			return nil, err
		}
		return &extractorNode{
			name:    nd.Name,
			element: elem,
			ext:     ext,
			meta:    nd.Meta,
		}, nil

	case HandlerTypeRenderer:
		rnd, err := resolveRenderer(handler, nd, reg)
		if err != nil {
			return nil, err
		}
		return &rendererNode{
			name:    nd.Name,
			element: elem,
			rnd:     rnd,
			meta:    nd.Meta,
		}, nil

	case HandlerTypeNode:
		if reg.Nodes == nil {
			return nil, fmt.Errorf("node %q: handler %q not found (node registry is nil)", nd.Name, handler)
		}
		factory, ok := reg.Nodes[handler]
		if !ok {
			return nil, fmt.Errorf("node %q: handler %q not found in node registry", nd.Name, handler)
		}
		return factory(*nd), nil

	case HandlerTypeDelegate:
		if reg.Transformers == nil {
			return nil, fmt.Errorf("node %q: delegate handler %q not found (transformer registry is nil)", nd.Name, handler)
		}
		gen, err := reg.Transformers.Get(handler)
		if err != nil {
			return nil, fmt.Errorf("node %q: delegate handler: %w", nd.Name, err)
		}
		return &dslDelegateNode{
			name:       nd.Name,
			element:    elem,
			gen:        gen,
			config:     def.Vars,
			nodeConfig: nd.EffectiveConfig(),
			meta:       nd.Meta,
		}, nil

	case HandlerTypeCircuit:
		slog.DebugContext(context.Background(), "resolve circuit handler",
			"component", "build",
			"node", nd.Name,
			"handler", handler,
			"circuits_nil", reg.Circuits == nil,
			"circuits_count", len(reg.Circuits),
			"mediator_endpoint", reg.MediatorEndpoint,
		)
		if reg.Circuits != nil {
			if cd, ok := reg.Circuits[handler]; ok {
				slog.DebugContext(context.Background(), "circuit handler resolved locally",
					"component", "build",
					"node", nd.Name,
					"handler", handler,
				)
				return &circuitRefNode{
					name:       nd.Name,
					element:    elem,
					circuitDef: cd,
					meta:       nd.Meta,
				}, nil
			}
		}
		if reg.MediatorEndpoint != "" {
			slog.DebugContext(context.Background(), "circuit handler delegating to mediator",
				"component", "build",
				"node", nd.Name,
				"handler", handler,
				"endpoint", reg.MediatorEndpoint,
			)
			return &transformerNode{
				name:       nd.Name,
				element:    elem,
				trans:      &MCPCircuitTransformer{CircuitType: handler, Endpoint: reg.MediatorEndpoint},
				config:     def.Vars,
				nodeConfig: nd.EffectiveConfig(),
				meta:       nd.Meta,
			}, nil
		}
		return nil, fmt.Errorf("node %q: circuit handler %q not found (no local circuit and no mediator endpoint)", nd.Name, handler)

	default:
		return nil, fmt.Errorf("node %q: unknown handler_type %q", nd.Name, ht)
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
		return nil, fmt.Errorf("node %q: transformer %q not found (registry is nil)", nodeName, name)
	}
	return reg.Transformers.Get(name)
}

// dslEdge is a default Edge implementation created from an circuit.EdgeDef when
// no custom factory is registered. It always matches (returns a transition).
type dslEdge struct {
	def circuit.EdgeDef
}

func (e *dslEdge) ID() string       { return e.def.ID }
func (e *dslEdge) From() string     { return e.def.From }
func (e *dslEdge) To() string       { return e.def.To }
func (e *dslEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *dslEdge) IsLoop() bool     { return e.def.Loop }
func (e *dslEdge) IsParallel() bool { return e.def.Parallel }
func (e *dslEdge) Evaluate(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
	return &circuit.Transition{
		NextNode:    e.def.To,
		Explanation: e.def.Condition,
	}
}

// resolveExtractor resolves an extractor by name.
func resolveExtractor(def *circuit.CircuitDef, name string, nd *circuit.NodeDef, reg *GraphRegistries) (Extractor, error) {
	switch name {
	case BuiltinExtractorJSONSchema:
		return &JSONSchemaExtractor{Schema: nd.Schema}, nil
	case BuiltinExtractorRegex:
		cfg := nd.EffectiveConfig()
		pattern := cfg.Pattern
		if pattern == "" {
			return nil, fmt.Errorf("node %q: regex extractor requires meta.pattern", nd.Name)
		}
		return NewRegexExtractor(nd.Name, pattern)
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
				return nil, fmt.Errorf("extractor %q: regex type requires pattern", ed.Name)
			}
			return NewRegexExtractor(ed.Name, ed.Pattern)
		default:
			return nil, fmt.Errorf("extractor %q: unknown type %q", ed.Name, ed.Type)
		}
	}

	if reg.Extractors != nil {
		ext, err := reg.Extractors.Get(name)
		if err == nil {
			return ext, nil
		}
	}
	return nil, fmt.Errorf("node %q: extractor %q not found", nd.Name, name)
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
	return nil, fmt.Errorf("node %q: renderer %q not found", nd.Name, name)
}

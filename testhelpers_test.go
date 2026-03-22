package framework

// testhelpers_test.go provides test-only shims for types that moved to engine/.
// These let existing root-package tests compile without modification.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/finding"
)

// passthroughTransformer recreates the built-in passthrough for tests.
type passthroughTransformer struct{}

func (t *passthroughTransformer) Name() string        { return "passthrough" }
func (t *passthroughTransformer) Deterministic() bool { return true }
func (t *passthroughTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Input, nil
}

// goTemplateTransformer recreates the built-in go-template for tests.
type goTemplateTransformer struct{}

func (t *goTemplateTransformer) Name() string        { return "go-template" }
func (t *goTemplateTransformer) Deterministic() bool { return true }
func (t *goTemplateTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Prompt, nil
}

// circuitRefNode is a DelegateNode that references a pre-loaded CircuitDef.
type circuitRefNode struct {
	name       string
	element    Element
	circuitDef *CircuitDef
	meta       map[string]any
}

func (n *circuitRefNode) Name() string            { return n.name }
func (n *circuitRefNode) ElementAffinity() Element { return n.element }
func (n *circuitRefNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return &DelegateArtifact{GeneratedCircuit: n.circuitDef, NodeCount: len(n.circuitDef.Nodes)}, nil
}
func (n *circuitRefNode) GenerateCircuit(_ context.Context, _ NodeContext) (*CircuitDef, error) {
	return n.circuitDef, nil
}

// dslDelegateNode recreates the DSL delegate node for tests.
type dslDelegateNode struct {
	name    string
	element Element
	gen     Transformer
	config  map[string]any
	meta    map[string]any
}

func (n *dslDelegateNode) Name() string            { return n.name }
func (n *dslDelegateNode) ElementAffinity() Element { return n.element }

func (n *dslDelegateNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	da, err := n.GenerateCircuit(ctx, nc)
	if err != nil {
		return nil, err
	}
	return &DelegateArtifact{GeneratedCircuit: da, NodeCount: len(da.Nodes)}, nil
}

func (n *dslDelegateNode) GenerateCircuit(ctx context.Context, nc NodeContext) (*CircuitDef, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	tc := &TransformerContext{
		Input:       input,
		Config:      n.config,
		NodeName:    n.name,
		Meta:        n.meta,
		WalkerState: nc.WalkerState,
	}
	result, err := n.gen.Transform(ctx, tc)
	if err != nil {
		return nil, err
	}
	switch v := result.(type) {
	case *CircuitDef:
		return v, nil
	case CircuitDef:
		return &v, nil
	default:
		return nil, nil
	}
}

// transformerArtifact wraps transformer output. Test-only shim.
type transformerArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *transformerArtifact) Type() string       { return a.typeName }
func (a *transformerArtifact) Confidence() float64 { return a.confidence }
func (a *transformerArtifact) Raw() any            { return a.raw }

// extractorNode is a Node that delegates to an Extractor. Test-only shim.
type extractorNode struct {
	name    string
	element Element
	ext     Extractor
	meta    map[string]any
}

func (n *extractorNode) Name() string            { return n.name }
func (n *extractorNode) ElementAffinity() Element { return n.element }
func (n *extractorNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	result, err := n.ext.Extract(ctx, input)
	if err != nil {
		return nil, err
	}
	return &extractorArtifact{typeName: n.ext.Name(), confidence: 1.0, raw: result}, nil
}

// extractorArtifact wraps extractor output. Test-only shim.
type extractorArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *extractorArtifact) Type() string       { return a.typeName }
func (a *extractorArtifact) Confidence() float64 { return a.confidence }
func (a *extractorArtifact) Raw() any            { return a.raw }

// rendererArtifact wraps renderer output. Test-only shim.
type rendererArtifact struct {
	typeName   string
	confidence float64
	raw        string
}

func (a *rendererArtifact) Type() string       { return a.typeName }
func (a *rendererArtifact) Confidence() float64 { return a.confidence }
func (a *rendererArtifact) Raw() any            { return a.raw }

// transformerNode is a test-only stub matching the engine/ internal type shape.
type transformerNode struct {
	name     string
	element  Element
	trans    Transformer
	prompt   string
	input    string
	provider string
	config   map[string]any
	meta     map[string]any
}

func (n *transformerNode) Name() string            { return n.name }
func (n *transformerNode) ElementAffinity() Element { return n.element }
func (n *transformerNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	var input any
	if n.input != "" {
		resolved, err := ResolveInput(n.input, nc.WalkerState.Outputs)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			input = resolved.Raw()
		}
	} else if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	prompt := n.prompt
	if prompt != "" {
		sources := make(map[string]any)
		if nc.WalkerState != nil && nc.WalkerState.Outputs != nil {
			for k, v := range nc.WalkerState.Outputs {
				sources[k] = v.Raw()
			}
		}
		tmplCtx := TemplateContext{
			Output:  input,
			State:   nc.WalkerState,
			Config:  n.config,
			Sources: sources,
			Node:    n.name,
		}
		rendered, err := RenderPrompt(prompt, tmplCtx)
		if err != nil {
			return nil, err
		}
		prompt = rendered
	}
	meta := nc.Meta
	if meta == nil {
		meta = make(map[string]any)
	}
	for k, v := range n.meta {
		meta[k] = v
	}
	if n.provider != "" {
		meta["provider"] = n.provider
	}
	tc := &TransformerContext{
		Input:       input,
		Config:      n.config,
		Prompt:      prompt,
		NodeName:    n.name,
		Meta:        meta,
		WalkerState: nc.WalkerState,
	}
	if typed, ok := n.trans.(TypedTransformer); ok {
		if expected := typed.InputType(); expected != nil {
			if tc.Input == nil {
				return nil, fmt.Errorf("node %s: expected input type %s but got nil", tc.NodeName, expected)
			}
			actual := reflect.TypeOf(tc.Input)
			if !actual.AssignableTo(expected) {
				return nil, fmt.Errorf("node %s: input type %s not assignable to expected %s", tc.NodeName, actual, expected)
			}
		}
	}
	result, err := n.trans.Transform(ctx, tc)
	if err != nil {
		return nil, err
	}
	return &transformerArtifact{typeName: n.trans.Name(), confidence: 1.0, raw: result}, nil
}

// dslEdge is a default edge. Test-only shim.
type dslEdge struct {
	def EdgeDef
}

func (e *dslEdge) ID() string       { return e.def.ID }
func (e *dslEdge) From() string     { return e.def.From }
func (e *dslEdge) To() string       { return e.def.To }
func (e *dslEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *dslEdge) IsLoop() bool     { return e.def.Loop }
func (e *dslEdge) IsParallel() bool { return e.def.Parallel }
func (e *dslEdge) Evaluate(_ Artifact, _ *WalkerState) *Transition {
	return &Transition{
		NextNode:    e.def.To,
		Explanation: e.def.Condition,
	}
}

// buildExprContext and artifactToMap forward to engine/ exported test helpers.
func buildExprContext(artifact Artifact, state *WalkerState, config map[string]any) ExprContext {
	return engine.BuildExprContextForTest(artifact, state, config)
}

func artifactToMap(artifact Artifact) map[string]any {
	return engine.ArtifactToMapForTest(artifact)
}

// runExprProgram forwards to engine/ for expression edge tests.
var runExprProgram = engine.RunExprProgramForTest

// evidenceSNR computes signal-to-noise ratio. Test-only shim.
func evidenceSNR(inputItems, outputItems int) float64 {
	if inputItems <= 0 {
		return 0
	}
	return float64(outputItems) / float64(inputItems)
}

// hookingWalker is a test-only shim wrapping a Walker with hook execution.
// Implements the same veto logic as engine/.
type hookingWalker struct {
	inner       Walker
	nodeBefore  map[string][]string
	nodeHooks   map[string][]string
	hooks       HookRegistry
	onHookEvent func(name, phase string, err error)
}

func (hw *hookingWalker) Identity() AgentIdentity     { return hw.inner.Identity() }
func (hw *hookingWalker) SetIdentity(id AgentIdentity) { hw.inner.SetIdentity(id) }
func (hw *hookingWalker) State() *WalkerState          { return hw.inner.State() }

func (hw *hookingWalker) Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error) {
	hookCtx := WithWalkerState(ctx, hw.State())
	for _, name := range hw.nodeBefore[node.Name()] {
		hook, hErr := hw.hooks.Get(name)
		if hErr != nil {
			continue
		}
		_ = hook.Run(hookCtx, node.Name(), nil)
	}

	artifact, err := hw.inner.Handle(ctx, node, nc)
	if err != nil {
		return nil, err
	}

	for _, name := range hw.nodeHooks[node.Name()] {
		hook, hErr := hw.hooks.Get(name)
		if hErr != nil {
			continue
		}
		if hErr = hook.Run(hookCtx, node.Name(), artifact); hErr != nil {
			if errors.Is(hErr, ErrFindingVeto) {
				artifact = &finding.VetoArtifact{Inner: artifact}
				continue
			}
		}
	}

	return artifact, nil
}

// artifactCaptureObserver captures EventNodeExit artifacts. Test-only shim.
type artifactCaptureObserver struct {
	store         *ArtifactStore
	observedNodes map[string]bool
	inner         WalkObserver
}

func (o *artifactCaptureObserver) OnEvent(e WalkEvent) {
	if e.Type == EventNodeExit && e.Artifact != nil {
		if len(o.observedNodes) == 0 || o.observedNodes[e.Node] {
			o.store.Set(e.Node, e.Artifact)
		}
	}
	if o.inner != nil {
		o.inner.OnEvent(e)
	}
}

// mcpCircuitTransformer aliases to engine.MCPCircuitTransformer for test backward compatibility.
type mcpCircuitTransformer = engine.MCPCircuitTransformer

// rendererNode test-only shim.
type rendererNode struct {
	name    string
	element Element
	rnd     Renderer
	meta    map[string]any
}

func (n *rendererNode) Name() string            { return n.name }
func (n *rendererNode) ElementAffinity() Element { return n.element }
func (n *rendererNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	result, err := n.rnd.Render(ctx, input)
	if err != nil {
		return nil, err
	}
	return &rendererArtifact{typeName: n.rnd.Name(), confidence: 1.0, raw: result}, nil
}

// zoneForNode finds the zone containing a node, or nil. Test-only shim.
func zoneForNode(nodeName string, zones []Zone) *Zone {
	for i := range zones {
		for _, n := range zones[i].NodeNames {
			if n == nodeName {
				return &zones[i]
			}
		}
	}
	return nil
}

// thermalObserver test-only shim.
type thermalObserver struct {
	inner   WalkObserver
	warning time.Duration
	ceiling time.Duration
	cancel  context.CancelFunc
	mu      sync.Mutex
	total   time.Duration
	warned  bool
	aborted bool
}

func (t *thermalObserver) OnEvent(e WalkEvent) {
	if t.inner != nil {
		t.inner.OnEvent(e)
	}
	if e.Type != EventNodeExit || e.Error != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.total += e.Elapsed
	if !t.warned && t.warning > 0 && t.total >= t.warning {
		t.warned = true
		emitEvent(t.inner, WalkEvent{
			Type: EventThermalWarning,
			Metadata: map[string]any{
				"cumulative": t.total.Seconds(),
				"warning":    t.warning.Seconds(),
				"ceiling":    t.ceiling.Seconds(),
			},
		})
	}
	if !t.aborted && t.total >= t.ceiling {
		t.aborted = true
		t.cancel()
	}
}

func (t *thermalObserver) Total() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

package engine

// Category: Processing & Support — renderer types.

import (
	"context"
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// Renderer converts structured data into human-readable output.
// Symmetric counterpart to Extractor (ADC/DAC duality):
// Extractor: unstructured -> structured, Renderer: structured -> unstructured.
type Renderer interface {
	Name() string
	Render(ctx context.Context, data any) (string, error)
}

// RendererRegistry maps renderer names to Renderer implementations.
type RendererRegistry map[string]Renderer

// Get returns the renderer registered under name, or an error if not found.
// Supports FQCN resolution identical to ExtractorRegistry.
func (r RendererRegistry) Get(name string) (Renderer, error) {
	if r == nil {
		return nil, ErrRendererRegistryIsNil
	}
	if rnd, ok := r[name]; ok {
		return rnd, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, rnd := range r {
			if strings.HasSuffix(k, suffix) {
				return rnd, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %q not registered", ErrRenderer, name)
}

// Register adds a renderer to the registry. Panics on duplicate name.
func (r RendererRegistry) Register(rnd Renderer) {
	if _, exists := r[rnd.Name()]; exists {
		panic(fmt.Sprintf("duplicate renderer registration: %q", rnd.Name()))
	}
	r[rnd.Name()] = rnd
}

// BuiltinRendererTemplate is the built-in renderer recognized by resolveNode.
const BuiltinRendererTemplate = "template"

// TemplateRenderer is the built-in renderer that wraps RenderPrompt.
type TemplateRenderer struct {
	Template string
}

func (t *TemplateRenderer) Name() string { return BuiltinRendererTemplate }

func (t *TemplateRenderer) Render(_ context.Context, data any) (string, error) {
	tc, ok := data.(circuit.TemplateContext)
	if !ok {
		return "", fmt.Errorf("%w: %T", ErrTemplateRendererExpectedCircuitTemplateContextGot, data)
	}
	return RenderPrompt(t.Template, tc)
}

// rendererNode is a Node that delegates processing to a Renderer.
type rendererNode struct {
	name    string
	element circuit.Element
	rnd     Renderer
}

func (n *rendererNode) Name() string                     { return n.name }
func (n *rendererNode) ElementAffinity() circuit.Element { return n.element }

func (n *rendererNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	result, err := n.rnd.Render(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("renderer %q: %w", n.rnd.Name(), err)
	}
	return &rendererArtifact{
		typeName:   n.rnd.Name(),
		confidence: 1.0,
		raw:        result,
	}, nil
}

// rendererArtifact wraps the output of a Renderer as an Artifact.
type rendererArtifact struct {
	typeName   string
	confidence float64
	raw        string
}

func (a *rendererArtifact) Type() string        { return a.typeName }
func (a *rendererArtifact) Confidence() float64 { return a.confidence }
func (a *rendererArtifact) Raw() any            { return a.raw }

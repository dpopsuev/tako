package engine

// Category: Processing & Support — renderer types.

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/identity"
)

// Renderer converts structured data into human-readable output.
// Renderer, RendererRegistry are defined in engine/handler.

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
	element identity.Element
	rnd     Renderer
}

func (n *rendererNode) Name() string                      { return n.name }
func (n *rendererNode) ElementAffinity() identity.Element { return n.element }

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

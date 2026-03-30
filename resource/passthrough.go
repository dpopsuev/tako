package resource

import (
	"github.com/dpopsuev/origami/circuit"

	"gopkg.in/yaml.v3"
)

// RawResource is the parsed result of a passthrough handler.
// It holds the envelope fields plus the full YAML as a map.
type RawResource struct {
	Kind    circuit.Kind   `json:"kind"`
	Version string         `json:"version,omitempty"`
	Name    string         `json:"name,omitempty"`
	Data    map[string]any `json:"data"`
}

// passthroughHandler parses any YAML with a kind: field into a RawResource.
// Used for domain kinds whose typed parsers live in consumer repos.
type passthroughHandler struct {
	kind circuit.Kind
}

// NewPassthroughHandler creates a handler that parses envelope + raw YAML map.
// Consumers can register typed handlers to override these.
func NewPassthroughHandler(kind circuit.Kind) KindHandler {
	return &passthroughHandler{kind: kind}
}

func (h *passthroughHandler) Kind() circuit.Kind { return h.kind }

func (h *passthroughHandler) Parse(data []byte) (any, error) {
	env, err := circuit.ParseEnvelope(data)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &RawResource{
		Kind:    env.Kind,
		Version: env.Version,
		Name:    env.Metadata.Name,
		Data:    raw,
	}, nil
}

func (h *passthroughHandler) Validate(_ any) error        { return nil }
func (h *passthroughHandler) Merge(_, _ any) (any, error) { return nil, ErrMergeNotSupported }
func (h *passthroughHandler) SupportsMerge() bool         { return false }

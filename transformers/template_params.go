package transformers

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// TemplateParamsTransformer assembles template parameters from walker state,
// config, and metadata. This is the DSL-first replacement for hand-coded
// BuildParams() functions: declare data sources in YAML instead of Go.
//
// Usage in circuit YAML:
//
//	nodes:
//	  - name: build-context
//	    transformer: template-params
//	    meta:
//	      include_state: true    # merge walker state outputs
//	      include_config: true   # merge circuit vars
//	      extra:                 # static key-value pairs
//	        step: recall
type TemplateParamsTransformer struct{}

// NewTemplateParams creates a template-params transformer.
func NewTemplateParams() *TemplateParamsTransformer { return &TemplateParamsTransformer{} }

const transformerNameTemplateParams = "template-params"

func (t *TemplateParamsTransformer) Name() string        { return transformerNameTemplateParams }
func (t *TemplateParamsTransformer) Deterministic() bool { return true }

func (t *TemplateParamsTransformer) Transform(_ context.Context, tc *engine.TransformerContext) (any, error) {
	params := make(map[string]any)

	cfg := tc.NodeConfig
	if cfg == nil {
		cfg = &circuit.NodeConfig{}
	}

	if cfg.IncludeState && tc.Input != nil {
		if m, ok := tc.Input.(map[string]any); ok {
			for k, v := range m {
				params[k] = v
			}
		} else {
			params["input"] = tc.Input
		}
	}

	if cfg.IncludeConfig {
		for k, v := range tc.Config {
			params[k] = v
		}
	}

	for k, v := range cfg.Extra {
		params[k] = v
	}

	if len(cfg.Pick) > 0 {
		picked := make(map[string]any, len(cfg.Pick))
		for _, key := range cfg.Pick {
			if v, exists := params[key]; exists {
				picked[key] = v
			}
		}
		return picked, nil
	}

	params["node"] = tc.NodeName
	return params, nil
}

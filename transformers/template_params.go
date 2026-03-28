package transformers

import (
	"context"
	"fmt"

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

	if includeState, _ := tc.Meta["include_state"].(bool); includeState && tc.Input != nil {
		if m, ok := tc.Input.(map[string]any); ok {
			for k, v := range m {
				params[k] = v
			}
		} else {
			params["input"] = tc.Input
		}
	}

	if includeConfig, _ := tc.Meta["include_config"].(bool); includeConfig {
		for k, v := range tc.Config {
			params[k] = v
		}
	}

	if extra, ok := tc.Meta["extra"].(map[string]any); ok {
		for k, v := range extra {
			params[k] = v
		}
	}

	if keys, ok := tc.Meta["pick"].([]any); ok {
		picked := make(map[string]any, len(keys))
		for _, k := range keys {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("template-params: pick key must be string, got %T", k)
			}
			if v, exists := params[key]; exists {
				picked[key] = v
			}
		}
		return picked, nil
	}

	params["node"] = tc.NodeName
	return params, nil
}

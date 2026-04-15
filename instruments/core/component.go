package transformers

import (
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"
)

// CoreComponent returns a Component bundling the four built-in transformers
// (llm, http, jq, file) under the "core" namespace.
// The llm transformer requires a Dispatcher; pass nil to omit it.
func CoreComponent(d dispatch.Dispatcher, opts ...CoreComponentOption) *engine.Component {
	cfg := &coreComponentConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	reg := engine.TransformerRegistry{}
	if d != nil {
		var llmOpts []LLMOption
		if cfg.baseDir != "" {
			llmOpts = append(llmOpts, WithBaseDir(cfg.baseDir))
		}
		reg["llm"] = NewLLM(d, llmOpts...)
	}
	reg["http"] = NewHTTP()
	reg["jq"] = NewJQ()

	var fileOpts []FileOption
	if cfg.baseDir != "" {
		fileOpts = append(fileOpts, WithRootDir(cfg.baseDir))
	}
	reg["file"] = NewFile(fileOpts...)
	reg["template-params"] = NewTemplateParams()
	reg["match"] = NewMatch()

	return &engine.Component{
		Namespace:    "core",
		Name:         "origami-core",
		Version:      "1.0.0",
		Description:  "Built-in transformers: llm, http, jq, file",
		Transformers: reg,
	}
}

// CoreComponentOption configures CoreComponent.
type CoreComponentOption func(*coreComponentConfig)

type coreComponentConfig struct {
	baseDir string
}

// WithCoreBaseDir sets the base directory for file and llm transformers.
func WithCoreBaseDir(dir string) CoreComponentOption {
	return func(c *coreComponentConfig) { c.baseDir = dir }
}

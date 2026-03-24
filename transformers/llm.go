// Package transformers provides built-in Transformer implementations for the
// Origami DSL. These are the "batteries included" that require zero custom code.
package transformers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bd "github.com/dpopsuev/bugle/dispatch"
	"github.com/dpopsuev/origami/engine"
)

// LLMTransformer sends a prompt to an external agent via a Dispatcher and
// parses the JSON response. This is the primary transformer for AI-driven
// circuit nodes.
type LLMTransformer struct {
	dispatcher bd.Dispatcher
	baseDir    string // base directory for resolving prompt/artifact paths
}

// LLMOption configures the LLM transformer.
type LLMOption func(*LLMTransformer)

// WithBaseDir sets the base directory for resolving relative paths.
func WithBaseDir(dir string) LLMOption {
	return func(t *LLMTransformer) { t.baseDir = dir }
}

// NewLLM creates a transformer that dispatches prompts to an external agent.
func NewLLM(d bd.Dispatcher, opts ...LLMOption) *LLMTransformer {
	t := &LLMTransformer{dispatcher: d, baseDir: "."}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *LLMTransformer) Name() string        { return "llm" }
func (t *LLMTransformer) Deterministic() bool { return false }

func (t *LLMTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	promptPath := tc.Prompt
	if promptPath != "" && !filepath.IsAbs(promptPath) {
		promptPath = filepath.Join(t.baseDir, promptPath)
	}

	artifactPath := ""
	if tc.Meta != nil {
		if ap, ok := tc.Meta["artifact_path"].(string); ok {
			artifactPath = ap
		}
	}
	if artifactPath == "" {
		dir, err := os.MkdirTemp("", "origami-llm-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		artifactPath = filepath.Join(dir, "artifact.json")
		defer os.RemoveAll(dir)
	}

	caseID := ""
	if tc.Meta != nil {
		if c, ok := tc.Meta["case_id"].(string); ok {
			caseID = c
		}
	}

	dc := bd.Context{
		CaseID:       caseID,
		Step:         tc.NodeName,
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	data, err := t.dispatcher.Dispatch(ctx, dc)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", tc.NodeName, err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response for %s: %w", tc.NodeName, err)
	}

	return result, nil
}

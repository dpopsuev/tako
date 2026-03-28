package transformers

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/toolkit"
)

// Re-exports from toolkit for backward compatibility.
type (
	MatchRule      = toolkit.MatchRule
	MatchRuleSet   = toolkit.MatchRuleSet
	MatchEvaluator = toolkit.MatchEvaluator
)

var NewMatchEvaluator = toolkit.NewMatchEvaluator

// matchTransformer wraps a MatchEvaluator as a engine.Transformer.
// Config must include "rule_set" (string) and may include "field" (string)
// to select which input field to match against.
type matchTransformer struct{}

// NewMatch returns the match transformer for pipeline use.
func NewMatch() engine.Transformer {
	return &matchTransformer{}
}

const transformerNameMatch = "match"

func (t *matchTransformer) Name() string        { return transformerNameMatch }
func (t *matchTransformer) Deterministic() bool { return true }

func (t *matchTransformer) Transform(_ context.Context, tc *engine.TransformerContext) (any, error) {
	evaluator, _ := tc.Meta["evaluator"].(*MatchEvaluator)
	if evaluator == nil {
		return nil, fmt.Errorf("match transformer: no evaluator in meta")
	}

	ruleSetName, _ := tc.Meta["rule_set"].(string)
	if ruleSetName == "" {
		return nil, fmt.Errorf("match transformer: rule_set not specified in meta")
	}

	text, _ := tc.Input.(string)
	if text == "" {
		if m, ok := tc.Input.(map[string]any); ok {
			field, _ := tc.Meta["field"].(string)
			if field == "" {
				field = "text"
			}
			text, _ = m[field].(string)
		}
	}

	result, matched, err := evaluator.Evaluate(ruleSetName, text)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"result":  result,
		"matched": matched,
	}, nil
}

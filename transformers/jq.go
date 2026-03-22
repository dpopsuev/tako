package transformers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/origami/engine"
	"github.com/expr-lang/expr"
)

// JQTransformer reshapes JSON data using expr-lang expressions.
// The expression has access to the full input as `input` and config as `config`.
type JQTransformer struct{}

// NewJQ creates a transformer that evaluates expressions against input data.
func NewJQ() *JQTransformer { return &JQTransformer{} }

func (t *JQTransformer) Name() string        { return "jq" }
func (t *JQTransformer) Deterministic() bool { return true }

func (t *JQTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	expression, _ := metaString(tc, "expr")
	if expression == "" {
		return nil, fmt.Errorf("jq transformer: 'expr' is required in meta")
	}

	input := normalizeToMap(tc.Input)

	env := map[string]any{
		"input":  input,
		"config": tc.Config,
	}

	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("jq transformer: compile %q: %w", expression, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("jq transformer: eval %q: %w", expression, err)
	}

	return result, nil
}

func normalizeToMap(v any) any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var m any
	if err := json.Unmarshal(data, &m); err != nil {
		return v
	}
	return m
}

package fab

import (
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/tako/artifact"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type PredicateContext struct {
	Output map[string]any `expr:"output"`
	Labels map[string]string `expr:"labels"`
}

type PredicateEvaluator struct {
	expression string
	program    *vm.Program
}

var _ ContractEvaluator = (*PredicateEvaluator)(nil)

func NewPredicate(expression string) (*PredicateEvaluator, error) {
	program, err := expr.Compile(expression,
		expr.Env(PredicateContext{}),
		expr.AsBool(),
	)
	if err != nil {
		return nil, fmt.Errorf("fab: compile predicate %q: %w", expression, err)
	}
	return &PredicateEvaluator{expression: expression, program: program}, nil
}

func MustPredicate(expression string) *PredicateEvaluator {
	p, err := NewPredicate(expression)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *PredicateEvaluator) Evaluate(_ Contract, env artifact.Envelope) (bool, error) {
	var output map[string]any
	if err := json.Unmarshal(env.Payload, &output); err != nil {
		return false, fmt.Errorf("fab: unmarshal payload for predicate: %w", err)
	}

	ctx := PredicateContext{
		Output: output,
		Labels: env.Labels,
	}

	result, err := expr.Run(p.program, ctx)
	if err != nil {
		return false, fmt.Errorf("fab: evaluate predicate %q: %w", p.expression, err)
	}

	matched, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("fab: predicate %q returned %T, expected bool", p.expression, result)
	}
	return matched, nil
}

func (p *PredicateEvaluator) Expression() string {
	return p.expression
}

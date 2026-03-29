package toolkit

import "errors"

var (
	// ErrNoPromptFilenameForNode is returned for: no prompt filename for node
	ErrNoPromptFilenameForNode = errors.New("no prompt filename for node")

	// ErrMatchEvaluatorRuleSet is returned for: match evaluator: rule set
	ErrMatchEvaluatorRuleSet = errors.New("match evaluator: rule set")

	// ErrStep is returned for: step
	ErrStep = errors.New("step")
)

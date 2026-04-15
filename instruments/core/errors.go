package transformers

import "errors"

var (
	// ErrFileTransformerFilePathRequiredSetPromptOrConfigExtr is returned for: file transformer: file path required (set prompt: or config.extras.path)
	ErrFileTransformerFilePathRequiredSetPromptOrConfigExtr = errors.New("file transformer: file path required (set prompt: or config.extras.path)")

	// ErrFileTransformerPath is returned for: file transformer: path
	ErrFileTransformerPath = errors.New("file transformer: path")

	// ErrHttpTransformerUrlIsRequiredInNodeConfig is returned for: http transformer: 'url' is required in node config
	ErrHttpTransformerUrlIsRequiredInNodeConfig = errors.New("http transformer: 'url' is required in node config")

	// ErrHttpTransformerHostNotInAllowlistForUrl is returned for: http transformer: host not in allowlist for url
	ErrHttpTransformerHostNotInAllowlistForUrl = errors.New("http transformer: host not in allowlist for url")

	// ErrHttpTransformerStatus is returned for: http transformer: status
	ErrHttpTransformerStatus = errors.New("http transformer: status")

	// ErrJqTransformerExprIsRequiredInNodeConfig is returned for: jq transformer: 'expr' is required in node config
	ErrJqTransformerExprIsRequiredInNodeConfig = errors.New("jq transformer: 'expr' is required in node config")

	// ErrMatchTransformerNoNodeConfig is returned for: match transformer: no node config
	ErrMatchTransformerNoNodeConfig = errors.New("match transformer: no node config")

	// ErrMatchTransformerNoEvaluatorInConfig is returned for: match transformer: no evaluator in config
	ErrMatchTransformerNoEvaluatorInConfig = errors.New("match transformer: no evaluator in config")

	// ErrMatchTransformerRuleSetNotSpecifiedInConfig is returned for: match transformer: rule_set not specified in config
	ErrMatchTransformerRuleSetNotSpecifiedInConfig = errors.New("match transformer: rule_set not specified in config")
)

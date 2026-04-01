package engine

// TemplateParamsProvider declares which template parameters a consumer's
// circuit nodes expose. Used by lint rule P1 to validate that prompts
// reference only declared fields.
//
// Consumers implement this interface and register it via WithTemplateParams
// on SessionConfig.
type TemplateParamsProvider interface {
	// DeclaredParams returns the parameter names available for a given node.
	// Returns nil if the node has no declared parameters (no validation).
	DeclaredParams(nodeName string) []string
}

// StaticTemplateParams provides the same parameters for all nodes.
type StaticTemplateParams struct {
	Params []string
}

// DeclaredParams returns the static parameter list for any node.
func (s *StaticTemplateParams) DeclaredParams(_ string) []string {
	return s.Params
}

package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type TemplateContext = circuit.TemplateContext

func ResolveInput(input string, outputs map[string]Artifact) (Artifact, error) {
	return circuit.ResolveInput(input, outputs)
}

func RenderPrompt(tmplContent string, tc TemplateContext) (string, error) {
	return circuit.RenderPrompt(tmplContent, tc)
}

func MergeVars(base map[string]any, overrides map[string]any) map[string]any {
	return circuit.MergeVars(base, overrides)
}

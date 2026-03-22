package circuit

// Category: DSL & Build — variable resolution and prompt rendering.

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/dpopsuev/origami/core"
)

var refPattern = regexp.MustCompile(`\$\{(\w+)\.output\}`)

// ResolveInput resolves an input reference like "${recall.output}" against
// the outputs map collected during Walk. Returns nil if the reference is
// empty (no explicit input dependency).
func ResolveInput(input string, outputs map[string]core.Artifact) (core.Artifact, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	m := refPattern.FindStringSubmatch(input)
	if m == nil {
		return nil, fmt.Errorf("invalid input reference %q: expected ${node.output}", input)
	}

	nodeName := m[1]
	art, ok := outputs[nodeName]
	if !ok {
		return nil, fmt.Errorf("input reference ${%s.output}: node %q has not produced output yet", nodeName, nodeName)
	}
	return art, nil
}

// TemplateContext is the unified context available to prompt templates.
type TemplateContext struct {
	Output  any                // prior node's output (resolved from input: or prior artifact)
	State   *core.WalkerState  // walker state (loops, history, context)
	Config  map[string]any     // circuit vars
	Sources map[string]any     // named outputs from prior nodes
	Node    string             // current node name
}

// RenderPrompt renders a Go text/template string against a TemplateContext.
// Used to fill prompt templates with circuit data before sending to a transformer.
func RenderPrompt(tmplContent string, tc TemplateContext) (string, error) {
	t, err := template.New("prompt").Option("missingkey=zero").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, tc); err != nil {
		return "", fmt.Errorf("render prompt template: %w", err)
	}
	return buf.String(), nil
}

// MergeVars merges CLI overrides into circuit vars. Overrides take precedence.
func MergeVars(base map[string]any, overrides map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any)
	}
	result := make(map[string]any, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

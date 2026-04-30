package cerebrum

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type PromptContext struct {
	Phase     string
	Domain    string
	Need      string
	Atoms     map[string]int
	TotalMass int
}

const promptTemplate = `# Phase: {{.Phase}}
Domain: {{.Domain}}

## Need
{{.Need}}

{{- if gt .TotalMass 0}}

## Current State
{{range $phase, $mass := .Atoms}}{{if gt $mass 0}}- {{$phase}}: {{$mass}} atoms
{{end}}{{end}}
{{- end}}

## Response Format
Respond with JSON: {"atoms": [{"type": "<phase>", "taxonomy": "<phase.facet.domain>", "content": "<your response>"}], "tool_call": {"name": "<instrument>", "input": <json>}}
`

var compiledTemplate = template.Must(
	template.New("prompt").Option("missingkey=zero").Parse(promptTemplate),
)

func buildPrompt(m *reactivity.Molecule, need []byte, domain Domain) string {
	phase := m.Phase()

	ctx := PromptContext{
		Phase:     phase.String(),
		Domain:    domain.String(),
		Need:      string(need),
		TotalMass: m.TotalMass(),
	}

	ctx.Atoms = make(map[string]int)
	for _, at := range []reactivity.AtomType{
		reactivity.IntentAtom, reactivity.AssessmentAtom, reactivity.KnowledgeAtom,
		reactivity.ExpansionAtom, reactivity.ReductionAtom, reactivity.SelectionAtom,
		reactivity.ExecutionAtom, reactivity.AcclimationAtom, reactivity.RefinementAtom,
		reactivity.RetrospectionAtom,
	} {
		if mass := m.Mass(at); mass > 0 {
			ctx.Atoms[at.String()] = mass
		}
	}

	var buf strings.Builder
	if err := compiledTemplate.Execute(&buf, ctx); err != nil {
		return fmt.Sprintf("phase:%s mass:%d need:%s", phase, m.Mass(phase), string(need))
	}
	return buf.String()
}

func mergeVars(base, overrides map[string]any) map[string]any {
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

func renderTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("prompt").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt template: %w", err)
	}
	return buf.String(), nil
}

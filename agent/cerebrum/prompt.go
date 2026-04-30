package cerebrum

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
)

type PromptContext struct {
	Phase        string
	PhaseGuide   string
	Domain       string
	Need         string
	Recollected  []string
	Atoms        map[string]int
	TotalMass    int
	Instruments  []InstrumentInfo
	HasShell     bool
}

type InstrumentInfo struct {
	Name        string
	Description string
}

const promptTemplate = `# Phase: {{.Phase}}
Domain: {{.Domain}}

{{.PhaseGuide}}

## Need
{{.Need}}

{{- if .Recollected}}

## Prior Knowledge
{{range .Recollected}}- {{.}}
{{end}}
{{- end}}

{{- if gt .TotalMass 0}}

## Current State
{{range $phase, $mass := .Atoms}}{{if gt $mass 0}}- {{$phase}}: {{$mass}} atoms
{{end}}{{end}}
{{- end}}

{{- if .HasShell}}

## Available Instruments
{{range .Instruments}}- {{.Name}}: {{.Description}}
{{end}}
To use an instrument, include a tool_call in your response.
{{- end}}

## Response Format
Respond with JSON: {"atoms": [{"type": "<phase>", "taxonomy": "<phase.facet.domain>", "content": "<your response>"}]{{if .HasShell}}, "tool_call": {"name": "<instrument>", "input": <json>}{{end}}}
`

var compiledTemplate = template.Must(
	template.New("prompt").Option("missingkey=zero").Parse(promptTemplate),
)

var phaseGuides = map[reactivity.AtomType]string{
	reactivity.IntentAtom:        "Determine what needs to be done. Identify the goal, constraints, and desired outcome.",
	reactivity.AssessmentAtom:    "Assess the situation. What do you know? What don't you know? What resources are available?",
	reactivity.PlanAtom:          "Create a plan. What steps are needed? In what order? What could go wrong?",
	reactivity.ExecutionAtom:     "Execute the plan. Use available instruments. Report results.",
	reactivity.RetrospectionAtom: "Reflect on what happened. What worked? What didn't? What would you do differently?",
}

func buildPrompt(m *reactivity.Molecule, need []byte, domain Domain, shell instrument.Shell, recollected []reactivity.Atom) string {
	phase := m.Phase()

	ctx := PromptContext{
		Phase:      phase.String(),
		PhaseGuide: phaseGuides[phase],
		Domain:     domain.String(),
		Need:       string(need),
		TotalMass:  m.TotalMass(),
		HasShell:   shell != nil && phase == reactivity.ExecutionAtom,
	}

	for _, a := range recollected {
		content := string(a.Content)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		ctx.Recollected = append(ctx.Recollected, content)
	}

	ctx.Atoms = make(map[string]int)
	for _, at := range []reactivity.AtomType{
		reactivity.IntentAtom, reactivity.AssessmentAtom, reactivity.PlanAtom,
		reactivity.ExecutionAtom, reactivity.RetrospectionAtom,
	} {
		if mass := m.Mass(at); mass > 0 {
			ctx.Atoms[at.String()] = mass
		}
	}

	if ctx.HasShell {
		for _, name := range shell.Names() {
			desc, _ := shell.Describe(name)
			ctx.Instruments = append(ctx.Instruments, InstrumentInfo{Name: name, Description: desc})
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

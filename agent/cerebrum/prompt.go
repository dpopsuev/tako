package cerebrum

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type PromptContext struct {
	Phase      string
	PhaseGuide string
	Domain     string
	Need       string
	Atoms      map[string]int
	TotalMass  int
}

const promptTemplate = `# Phase: {{.Phase}}
Domain: {{.Domain}}

{{.PhaseGuide}}

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

var phaseGuides = map[reactivity.AtomType]string{
	reactivity.IntentAtom:       "Determine what needs to be done. Identify the goal, constraints, and desired outcome.",
	reactivity.AssessmentAtom:   "Assess the situation. What do you know? What don't you know? What resources are available?",
	reactivity.KnowledgeAtom:    "", // deterministic: Recall (cache hit → mix intent+assessment+recalled) or Spill (cache miss → mix intent+assessment as-is)
	reactivity.ExpansionAtom:    "Expand all possible approaches. What are the options? Brainstorm without filtering.",
	reactivity.ReductionAtom:    "Reduce by identifying risks and problems for each approach. What could go wrong?",
	reactivity.SelectionAtom:    "Select the best approach based on ROI. Produce a plan graph.",
	reactivity.ExecutionAtom:    "Execute the plan. Use tool_call to invoke instruments. Report results.",
	reactivity.AcclimationAtom:  "Step back. Did we make the right decision? Is the result what we expected?",
	reactivity.RefinementAtom:   "Refine the approach based on observation. What adjustments improve the outcome?",
	reactivity.RetrospectionAtom: "Reflect on the full cycle. What worked? What didn't? What stabilizers carry forward?",
}

func buildPrompt(m *reactivity.Molecule, need []byte, domain Domain) string {
	phase := m.Phase()

	ctx := PromptContext{
		Phase:     phase.String(),
		PhaseGuide: phaseGuides[phase],
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

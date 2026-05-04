package cerebrum

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type ContractView struct {
	Phase    string
	Contract string
	Active   bool
	Filled   bool
	Summary  string
}

type ContractPromptContext struct {
	Need            string
	Domain          string
	Contracts       []ContractView
	ActivePhase     string
	ActiveContract  string
	Instructions    string
	FilledContracts []ContractView
}

const contractPromptTemplate = `# Need
{{.Need}}

# Thinking Tree
{{range .Contracts}}{{if .Active}}>> {{.Phase}}: {{.Contract}}
{{else if .Filled}}   {{.Phase}}: [DONE] {{.Summary}}
{{else}}   {{.Phase}}: {{.Contract}}
{{end}}{{end}}
## Active: {{.ActivePhase}}
{{.ActiveContract}}

{{.Instructions}}

{{- if gt (len .FilledContracts) 0}}

## Completed Contracts
{{range .FilledContracts}}- **{{.Phase}}**: {{.Summary}}
{{end}}{{end}}

## Response Format
Respond with JSON: {"atoms": [{"type": "<phase>", "taxonomy": "<phase.domain>", "content": "<your answer to the contract>"}]}
`

var contractTemplate = template.Must(
	template.New("contract").Option("missingkey=zero").Parse(contractPromptTemplate),
)

func buildContractPrompt(m *reactivity.Molecule, need []byte, domain Domain, contracts []reactivity.ContractInfo) string {
	phase := m.Phase()

	ctx := ContractPromptContext{
		Need:   string(need),
		Domain: domain.String(),
	}

	for _, c := range contracts {
		filled := m.Mass(c.Phase) > 0
		active := c.Phase == phase
		var summary string
		if filled {
			atoms := m.Atoms(c.Phase)
			if len(atoms) > 0 {
				summary = truncate(string(atoms[0].Content), 120)
			}
		}
		cv := ContractView{
			Phase:    c.Phase.String(),
			Contract: c.Contract,
			Active:   active,
			Filled:   filled,
			Summary:  summary,
		}
		ctx.Contracts = append(ctx.Contracts, cv)
		if active {
			ctx.ActivePhase = c.Phase.String()
			ctx.ActiveContract = c.Contract
		}
		if filled {
			ctx.FilledContracts = append(ctx.FilledContracts, cv)
		}
	}

	ctx.Instructions = instructionsForPhase(phase)

	var buf strings.Builder
	if err := contractTemplate.Execute(&buf, ctx); err != nil {
		return fmt.Sprintf("phase:%s need:%s", phase, string(need))
	}
	return buf.String()
}

func instructionsForPhase(phase reactivity.AtomType) string {
	switch phase.Triad {
	case reactivity.ThinkTriad:
		return "Fill this contract with reasoning. You may call instruments to gather information."
	case reactivity.ComposeTriad:
		return "Fill this contract with strategic analysis. Plan which instruments to use and in what order."
	case reactivity.ImplementTriad:
		if phase.Position == reactivity.ThesisPosition {
			return "Execute the committed plan NOW. Call ALL required instruments in this single response. Batch your tool calls."
		}
		return "Evaluate the execution results. Did the instruments respond as expected?"
	default:
		return "Evaluate the outcome. Was the need fulfilled?"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func buildPrompt(m *reactivity.Molecule, need []byte, domain Domain) string {
	return buildContractPrompt(m, need, domain, nil)
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

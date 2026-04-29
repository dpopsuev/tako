package cerebrum

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
)

var phaseInstructions = map[reactivity.AtomType]string{
	reactivity.IntentAtom:        "Determine what needs to be done. Identify the goal, constraints, and desired outcome.",
	reactivity.AssessmentAtom:    "Assess the situation. What do you know? What don't you know? What resources are available?",
	reactivity.PlanAtom:          "Create a plan. What steps are needed? In what order? What could go wrong?",
	reactivity.ExecutionAtom:     "Execute the plan. Use available instruments. Report results.",
	reactivity.RetrospectionAtom: "Reflect on what happened. What worked? What didn't? What would you do differently?",
}

func buildPrompt(m *reactivity.Molecule, need []byte, domain Domain, shell instrument.Shell, recollected []reactivity.Atom) string {
	var b strings.Builder

	phase := m.Phase()
	b.WriteString(fmt.Sprintf("# Phase: %s\n", phase))
	b.WriteString(fmt.Sprintf("Domain: %s\n\n", domain))

	if inst, ok := phaseInstructions[phase]; ok {
		b.WriteString(inst)
		b.WriteString("\n\n")
	}

	b.WriteString(fmt.Sprintf("## Need\n%s\n\n", string(need)))

	if len(recollected) > 0 {
		b.WriteString("## Prior Knowledge\n")
		for _, a := range recollected {
			content := string(a.Content)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			b.WriteString(fmt.Sprintf("- %s\n", content))
		}
		b.WriteString("\n")
	}

	if m.TotalMass() > 0 {
		b.WriteString("## Current State\n")
		for _, at := range []reactivity.AtomType{
			reactivity.IntentAtom, reactivity.AssessmentAtom, reactivity.PlanAtom,
			reactivity.ExecutionAtom, reactivity.RetrospectionAtom,
		} {
			mass := m.Mass(at)
			if mass > 0 {
				b.WriteString(fmt.Sprintf("- %s: %d atoms\n", at, mass))
			}
		}
		b.WriteString("\n")
	}

	if phase == reactivity.ExecutionAtom && shell != nil {
		b.WriteString("## Available Instruments\n")
		for _, name := range shell.Names() {
			desc, _ := shell.Describe(name)
			b.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
		}
		b.WriteString("\n")
		b.WriteString("To use an instrument, include a tool_call in your response.\n\n")
	}

	b.WriteString("## Response Format\n")
	b.WriteString("Respond with JSON: {\"atoms\": [{\"type\": \"<phase>\", \"taxonomy\": \"<phase.facet.domain>\", \"content\": \"<your response>\"}]")
	if phase == reactivity.ExecutionAtom && shell != nil {
		b.WriteString(", \"tool_call\": {\"name\": \"<instrument>\", \"input\": <json>}")
	}
	b.WriteString("}\n")

	return b.String()
}

package cerebrum

import (
	"fmt"
	"sort"
	"strings"
)

type Assembler interface {
	Assemble(ctx Context) string
}

func defaultRender(ctx Context) string {
	var b strings.Builder

	b.WriteString("# Need\n")
	b.WriteString(ctx.Need)
	b.WriteString("\n")

	if len(ctx.State) > 0 {
		b.WriteString("\n# Current State\n")
		writeMap(&b, ctx.State)
	}

	if len(ctx.Desired) > 0 {
		b.WriteString("\n# Desired State\n")
		writeMap(&b, ctx.Desired)
	}

	if len(ctx.Residual) > 0 {
		b.WriteString("\n# Gap\n")
		for _, k := range sortedKeys(ctx.Residual) {
			v := ctx.Residual[k]
			if v > 0 {
				b.WriteString(fmt.Sprintf("- %s: UNMET\n", k))
			} else {
				b.WriteString(fmt.Sprintf("- %s: met\n", k))
			}
		}
		trend := "stuck"
		if ctx.DeltaDistance < 0 {
			trend = "improving"
		} else if ctx.DeltaDistance > 0 {
			trend = "worsening"
		}
		b.WriteString(fmt.Sprintf("Distance: %.2f | Trend: %s\n", ctx.Distance, trend))
	}

	if len(ctx.Capabilities) > 0 {
		b.WriteString("\n# Available Actions\n")
		for _, cap := range ctx.Capabilities {
			if len(cap.Writes) > 0 {
				b.WriteString(fmt.Sprintf("- %s: %s [writes: %s]\n",
					cap.Name, cap.Description, strings.Join(cap.Writes, ", ")))
			}
		}
	}

	var activeContract string
	b.WriteString("\n# Thinking Tree\n")
	for _, c := range ctx.Contracts {
		filled := ctx.Filled[c.Phase.String()]
		active := c.Phase == ctx.Phase
		if active {
			b.WriteString(fmt.Sprintf(">> %s: %s\n", c.Phase, c.Contract))
			activeContract = c.Contract
		} else if filled != "" {
			b.WriteString(fmt.Sprintf("   %s: [DONE] %s\n", c.Phase, filled))
		} else {
			b.WriteString(fmt.Sprintf("   %s: %s\n", c.Phase, c.Contract))
		}
	}

	b.WriteString(fmt.Sprintf("\n## Active: %s\n", ctx.Phase))
	b.WriteString(activeContract)
	b.WriteString("\n\n")
	b.WriteString(instructionsForPhase(ctx.Phase))
	b.WriteString("\n")

	for _, d := range ctx.Directives {
		b.WriteString("\n> ")
		b.WriteString(string(d))
	}

	if len(ctx.Filled) > 0 {
		b.WriteString("\n\n## Completed\n")
		for _, c := range ctx.Contracts {
			if s := ctx.Filled[c.Phase.String()]; s != "" {
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", c.Phase, s))
			}
		}
	}

	b.WriteString("\n## Response Format\n")
	b.WriteString(`Respond with JSON: {"atoms": [{"type": "<phase>", "taxonomy": "<phase.domain>", "content": "<your answer to the contract>"}]}`)
	b.WriteString("\n")

	return b.String()
}

func writeMap(b *strings.Builder, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("- %s: %v\n", k, m[k]))
	}
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

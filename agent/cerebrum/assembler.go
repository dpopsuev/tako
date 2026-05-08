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
	renderNeed(&b, ctx)
	renderState(&b, ctx)
	renderDesired(&b, ctx)
	renderGap(&b, ctx)
	renderActions(&b, ctx)
	renderTree(&b, ctx)
	renderDirectives(&b, ctx)
	renderCompleted(&b, ctx)
	renderSight(&b, ctx)
	renderConstraints(&b)
	renderFormat(&b)
	return b.String()
}

func renderNeed(b *strings.Builder, ctx Context) {
	fmt.Fprintf(b, "# Need\n%s\n", ctx.Need)
}

func renderState(b *strings.Builder, ctx Context) {
	if len(ctx.StateChanges) > 0 {
		b.WriteString("\n# State Changes\n")
		for k, v := range ctx.StateChanges {
			fmt.Fprintf(b, "- %s: %v → %v\n", k, v[0], v[1])
		}
	} else if len(ctx.State) > 0 {
		b.WriteString("\n# Current State\n")
		writeMap(b, ctx.State)
	}
	if ctx.StagnantTurns > 1 {
		fmt.Fprintf(b, "\nWARNING: no progress for %d turns\n", ctx.StagnantTurns)
	}
}

func renderDesired(b *strings.Builder, ctx Context) {
	if len(ctx.Desired) == 0 {
		return
	}
	b.WriteString("\n# Desired State\n")
	writeMap(b, ctx.Desired)
}

func renderGap(b *strings.Builder, ctx Context) {
	if len(ctx.Residual) == 0 {
		return
	}
	b.WriteString("\n# Gap\n")
	for _, k := range sortedKeys(ctx.Residual) {
		if ctx.Residual[k] > 0 {
			fmt.Fprintf(b, "- %s: UNMET\n", k)
		} else {
			fmt.Fprintf(b, "- %s: met\n", k)
		}
	}
	trend := "stuck"
	if ctx.DeltaDistance < 0 {
		trend = "improving"
	} else if ctx.DeltaDistance > 0 {
		trend = "worsening"
	}
	fmt.Fprintf(b, "Distance: %.2f | Trend: %s\n", ctx.Distance, trend)
}

func renderActions(b *strings.Builder, ctx Context) {
	if len(ctx.Capabilities) == 0 {
		return
	}
	b.WriteString("\n# Available Actions\n")
	for _, cap := range ctx.Capabilities {
		if len(cap.Writes) > 0 {
			fmt.Fprintf(b, "- %s: %s [writes: %s]\n",
				cap.Name, cap.Description, strings.Join(cap.Writes, ", "))
		}
	}
}

func renderTree(b *strings.Builder, ctx Context) {
	var activeContract string
	b.WriteString("\n# Thinking Tree\n")
	for _, c := range ctx.Contracts {
		filled := ctx.Filled[c.Phase.String()]
		if c.Phase == ctx.Phase {
			fmt.Fprintf(b, ">> %s: %s\n", c.Phase, c.Contract)
			activeContract = c.Contract
		} else if filled != "" {
			fmt.Fprintf(b, "   %s: [DONE] %s\n", c.Phase, filled)
		} else {
			fmt.Fprintf(b, "   %s: %s\n", c.Phase, c.Contract)
		}
	}
	fmt.Fprintf(b, "\n## Active: %s\n%s\n\n%s\n",
		ctx.Phase, activeContract, instructionsForPhase(ctx.Phase))
}

func renderDirectives(b *strings.Builder, ctx Context) {
	for _, d := range ctx.Directives {
		fmt.Fprintf(b, "\n> %s", string(d))
	}
}

func renderCompleted(b *strings.Builder, ctx Context) {
	if len(ctx.Filled) == 0 {
		return
	}
	b.WriteString("\n\n## Completed\n")
	for _, c := range ctx.Contracts {
		if s := ctx.Filled[c.Phase.String()]; s != "" {
			fmt.Fprintf(b, "- **%s**: %s\n", c.Phase, s)
		}
	}
}

func renderSight(b *strings.Builder, ctx Context) {
	if sight := ctx.Sight.FormatPrompt(); sight != "" {
		fmt.Fprintf(b, "\n# Operator Focus\n%s\n", sight)
	}
}

var kissConstraints = []string{
	"No interfaces unless 2+ implementations exist right now",
	"Implement ONLY what the task describes, nothing extra",
	"Call things directly, no unnecessary wrapper layers",
	"Use stdlib first, import deps only when genuinely insufficient",
	"Write zero comments by default, one line max when WHY is non-obvious",
	"Trust internal code, only validate at system boundaries",
	"Delete unused code, don't rename to _ or add // deprecated",
}

func renderConstraints(b *strings.Builder) {
	b.WriteString("\n## Constraints (KISS)\n")
	for _, c := range kissConstraints {
		fmt.Fprintf(b, "- %s\n", c)
	}
}

func renderFormat(b *strings.Builder) {
	b.WriteString("\n## Response Format\n")
	fmt.Fprintf(b, "Respond with JSON: %s\n",
		`{"atoms": [{"type": "<phase>", "taxonomy": "<phase.domain>", "content": "<your answer to the contract>"}]}`)
}

func writeMap(b *strings.Builder, m map[string]any) {
	for _, k := range sortedMapKeys(m) {
		fmt.Fprintf(b, "- %s: %v\n", k, m[k])
	}
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}


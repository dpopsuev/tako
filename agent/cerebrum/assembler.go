package cerebrum

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type Assembler interface {
	Assemble(ctx Context) string
}

var KISSDirectives = []reactivity.Directive{
	"No interfaces unless 2+ implementations exist right now",
	"Implement ONLY what the task describes, nothing extra",
	"Call things directly, no unnecessary wrapper layers",
	"Use stdlib first, import deps only when genuinely insufficient",
	"Write zero comments by default, one line max when WHY is non-obvious",
	"Trust internal code, only validate at system boundaries",
	"Delete unused code, don't rename to _ or add // deprecated",
}

var promptTmpl = template.Must(template.New("prompt").Parse(promptText))

func defaultRender(ctx Context) string {
	hasDesired := len(ctx.Desired) > 0
	data := promptData{
		Need:          ctx.Need,
		State:         sortedKVs(ctx.State),
		StateChanges:  formatChanges(ctx.StateChanges),
		StagnantTurns: ctx.StagnantTurns,
		HasDesired:    hasDesired,
		Desired:       sortedKVs(ctx.Desired),
		Residual:      formatResidual(ctx.Residual),
		Distance:      ctx.Distance,
		Trend:         trend(ctx.DeltaDistance),
		Actions:       formatActions(ctx),
		Tree:          formatTree(ctx),
		ActivePhase:   ctx.Phase.String(),
		Instructions:  instructionsForPhase(ctx.Phase),
		Directives:    append(ctx.Directives, KISSDirectives...),
		Completed:     formatCompleted(ctx),
		Sight:         ctx.Sight.FormatPrompt(),
	}

	var buf bytes.Buffer
	if err := promptTmpl.Execute(&buf, data); err != nil {
		return "template error: " + err.Error()
	}
	return buf.String()
}

type promptData struct {
	Need          string
	State         []kv
	StateChanges  []string
	StagnantTurns int
	HasDesired    bool
	Desired       []kv
	Residual      []kv
	Distance      float64
	Trend         string
	Actions       []string
	Tree          []string
	ActivePhase   string
	Instructions  string
	Directives    []reactivity.Directive
	Completed     []string
	Sight         string
}

type kv struct{ Key, Value string }

func sortedKVs(m map[string]any) []kv {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]kv, len(keys))
	for i, k := range keys {
		out[i] = kv{k, fmt.Sprintf("%v", m[k])}
	}
	return out
}

func formatChanges(m map[string][2]any) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s: %v → %v", k, v[0], v[1]))
	}
	sort.Strings(out)
	return out
}

func formatResidual(m map[string]float64) []kv {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]kv, len(keys))
	for i, k := range keys {
		status := "met"
		if m[k] > 0 {
			status = "UNMET"
		}
		out[i] = kv{k, status}
	}
	return out
}

func trend(delta float64) string {
	switch {
	case delta < 0:
		return "improving"
	case delta > 0:
		return "worsening"
	default:
		return "stuck"
	}
}

func formatActions(ctx Context) []string {
	var out []string
	for _, cap := range ctx.Organs {
		if len(cap.Writes) > 0 {
			out = append(out, fmt.Sprintf("%s: %s [writes: %s]",
				cap.Name, cap.Description, strings.Join(cap.Writes, ", ")))
		}
	}
	return out
}

func formatTree(ctx Context) []string {
	var out []string
	for _, c := range ctx.Contracts {
		filled := ctx.Filled[c.Phase.String()]
		switch {
		case c.Phase == ctx.Phase:
			out = append(out, fmt.Sprintf(">> %s: %s", c.Phase, c.Contract))
		case filled != "":
			out = append(out, fmt.Sprintf("   %s: [DONE] %s", c.Phase, filled))
		default:
			out = append(out, fmt.Sprintf("   %s: %s", c.Phase, c.Contract))
		}
	}
	return out
}

func formatCompleted(ctx Context) []string {
	var out []string
	for _, c := range ctx.Contracts {
		if s := ctx.Filled[c.Phase.String()]; s != "" {
			out = append(out, fmt.Sprintf("**%s**: %s", c.Phase, s))
		}
	}
	return out
}

const promptText = `# Need
{{.Need}}
{{- if .StateChanges}}

# State Changes
{{- range .StateChanges}}
- {{.}}
{{- end}}
{{- else if .State}}

# Current State
{{- range .State}}
- {{.Key}}: {{.Value}}
{{- end}}
{{- end}}
{{- if gt .StagnantTurns 1}}

WARNING: no progress for {{.StagnantTurns}} turns
{{- end}}
{{- if .Desired}}

# Desired State
{{- range .Desired}}
- {{.Key}}: {{.Value}}
{{- end}}
{{- end}}
{{- if .Residual}}

# Gap
{{- range .Residual}}
- {{.Key}}: {{.Value}}
{{- end}}
Distance: {{printf "%.2f" .Distance}} | Trend: {{.Trend}}
{{- end}}
{{- if .Actions}}

# Available Actions
{{- range .Actions}}
- {{.}}
{{- end}}
{{- end}}
{{- if .HasDesired}}

# Thinking Tree
{{- range .Tree}}
{{.}}
{{- end}}

## Active: {{.ActivePhase}}
{{.Instructions}}
{{- if .Completed}}

## Completed
{{- range .Completed}}
- {{.}}
{{- end}}
{{- end}}

## Response Format
Respond with JSON: {"atoms": [{"type": "<phase>", "taxonomy": "<phase.domain>", "content": "<your answer to the contract>"}]}
{{- else}}

## Instructions
Use the available tools to fulfill the need. When the need is satisfied, respond with your final answer as plain text — do not call tools to deliver the response.
{{- end}}
{{- range .Directives}}

> {{.}}
{{- end}}
{{- if .Sight}}

# Operator Focus
{{.Sight}}
{{- end}}
`

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dpopsuev/origami/circuit"
)

func skillCmd(args []string) error {
	if len(args) == 0 {
		return ErrUsageOrigamiSkillScaffoldFlags
	}
	switch args[0] {
	case "scaffold":
		return skillScaffold(args[1:])
	default:
		return fmt.Errorf("%w: %s", ErrUnknownSkillSubcommand, args[0])
	}
}

func skillScaffold(args []string) error {
	fs := flag.NewFlagSet("skill scaffold", flag.ContinueOnError)
	toolName := fs.String("tool", "", "tool name (e.g. myapp, achilles)")
	outDir := fs.String("out", "", "output directory for SKILL.md (default: .cursor/skills/<tool>-calibrate/)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return ErrUsageOrigamiSkillScaffoldToolNAMEOutDIRCircuitYaml
	}
	circuitPath := fs.Arg(0)

	data, err := os.ReadFile(circuitPath)
	if err != nil {
		return fmt.Errorf("read circuit: %w", err)
	}

	def, err := circuit.LoadCircuit(data)
	if err != nil {
		return fmt.Errorf("parse circuit: %w", err)
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("validate circuit: %w", err)
	}

	tool := *toolName
	if tool == "" {
		tool = def.Circuit
	}

	dir := *outDir
	if dir == "" {
		dir = filepath.Join(".cursor", "skills", tool+"-calibrate")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(dir, "SKILL.md")

	ctx := scaffoldContext{
		Tool:        tool,
		CircuitName: def.Circuit,
		CircuitPath: circuitPath,
		Nodes:       def.Nodes,
		Edges:       def.Edges,
		Start:       string(def.Start),
		Done:        string(def.Done),
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()

	if err := skillTemplate.Execute(f, ctx); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	fmt.Fprintf(os.Stderr, "generated %s (%d nodes, %d edges)\n", outPath, len(def.Nodes), len(def.Edges))
	return nil
}

type scaffoldContext struct {
	Tool        string
	CircuitName string
	CircuitPath string
	Nodes       []circuit.NodeDef
	Edges       []circuit.EdgeDef
	Start       string
	Done        string
}

var funcMap = template.FuncMap{
	"upper": strings.ToUpper,
	"inc":   func(i int) int { return i + 1 },
}

var skillTemplate = template.Must(template.New("skill").Funcs(funcMap).Parse(`---
name: {{ .Tool }}-calibrate
description: >
  Run calibration for {{ .Tool }} via MCP. The Cursor agent supervises the
  {{ .CircuitName }} circuit: starts a session, launches worker subagents
  that independently pull steps and submit artifacts, monitors progress via
  signals, and presents the metrics report. Papercup v2 choreography pattern.
---

# {{ .Tool | upper }} Calibrate

Run calibration against a ground-truth scenario using the MCP server.

## Trigger

- ` + "`" + `/{{ .Tool }}-calibrate <SCENARIO>` + "`" + `
- ` + "`" + `/{{ .Tool }}-calibrate <SCENARIO> --parallel=4` + "`" + `

---

## Prerequisites

1. **MCP server** configured in ` + "`.cursor/mcp.json`" + `
2. **Binary built** — ` + "`" + `go build -o bin/{{ .Tool }} ./cmd/{{ .Tool }}/` + "`" + `

---

## Circuit Steps

| # | Node | Approach | Handler |
|---|------|---------|---------|
{{ range $i, $n := .Nodes -}}
| {{ inc $i }} | {{ $n.Name }} | {{ $n.Approach }} | {{ $n.Action }} |
{{ end }}
## Execution Flow

**Start node:** ` + "`" + `{{ .Start }}` + "`" + `
**Done node:** ` + "`" + `{{ .Done }}` + "`" + `

### Edges

{{ range .Edges -}}
- **{{ .Name }}** (` + "`" + `{{ .ID }}` + "`" + `): {{ .From }} → {{ .To }}{{ if .When }} when ` + "`" + `{{ .When }}` + "`" + `{{ end }}
{{ end }}

---

## Part 1 — Start calibration

Call the MCP tool:

` + "```" + `
start_calibration(
  scenario: "<SCENARIO>",
  backend: "cursor",
  parallel: 4,
  force: true
)
` + "```" + `

Store the returned ` + "`" + `session_id` + "`" + ` for all subsequent calls.

---

## Part 2 — Launch workers (Papercup v2)

You are the **supervisor**, not the executor. Launch N worker subagents
that each run an independent pull-process-submit loop. Do NOT call
` + "`" + `circuit(action: step)` + "`" + ` or ` + "`" + `circuit(action: submit)` + "`" + ` yourself — workers own those.

Launch up to 4 Task subagents in a **single message**. Each worker receives
the ` + "`" + `session_id` + "`" + ` and runs the worker loop below until the circuit completes.

### Worker loop (each subagent runs this independently)

` + "```" + `
signal(action: emit, session_id, "worker_started", "worker", meta={worker_id})
while true:
  response = circuit(action: step, session_id, timeout_ms: 30000)
  if response.done: break
  if not response.available: continue

  signal(action: emit, session_id, "start", "worker", response.case_id, response.step)
  prompt = read(response.prompt_path)
  artifact = generate_artifact(prompt)
  circuit(action: submit, session_id, artifact_json: artifact, dispatch_id: response.dispatch_id)
  signal(action: emit, session_id, "done", "worker", response.case_id, response.step, {bytes: size})
signal(action: emit, session_id, "worker_stopped", "worker", meta={worker_id})
` + "```" + `

Workers self-terminate when ` + "`" + `circuit(action: step)` + "`" + ` returns ` + "`" + `done=true` + "`" + `.
Fast workers immediately pull the next step — no waiting for slow siblings.

### Supervisor monitoring

While workers run, monitor progress via the signal bus:

` + "```" + `
signal(action: list, session_id, since=last_index)
` + "```" + `

If a worker emits an ` + "`" + `error` + "`" + ` signal and stops, launch a replacement worker.

---

## Part 3 — Report

Once all workers have stopped (all returned from their Task calls):

` + "```" + `
circuit(action: report, session_id)
` + "```" + `

Present the metrics scorecard to the user.

---

## Error handling

- **Worker failure:** worker emits error signal; supervisor detects via ` + "`" + `signal(action: list)` + "`" + ` and launches replacement.
- **Session timeout:** MCP server has a 5-minute inactivity watchdog. Workers keep it alive via ` + "`" + `circuit(action: submit)` + "`" + ` calls.
- **MCP disconnection:** session state is lost; re-run.

---

## Security guardrails

- Never echo API keys or credentials.
- Never read ground truth files during calibration.
- Workers must respect the calibration preamble in prompts.
`))

# Claude Code Instructions for Origami Development

## What is Origami

Origami is a circuit agent orchestration framework — YAML circuits executed by AI agents over MCP. Circuits declare nodes with instruments (compiled binaries or in-process functions), edges with conditions, and the engine walks them with parallel workers.

- Repo: github.com/dpopsuev/origami
- Scribe scope: origami
- Campaign: ORG-CMP-15 (active)

## Ecosystem Dependency Rules

**CRITICAL: Origami imports Troupe and Battery directly. No adapter layer — roster/agentport are dead.**

- Origami → Troupe (agent platform: signal bus, dispatch types)
- Origami → Battery (agent-world interface: tool types)
- Origami → Oculus (code analysis instrument)
- Djinn communicates with Origami via MCP protocol, not Go imports

Dependency direction: `Origami → Troupe ← Djinn` (lateral via MCP)

## Key Packages

```
circuit/          — Circuit types (re-exports from circuit/def)
circuit/def/      — Definition types (NodeDef, EdgeDef, Kind, Envelope)
circuit/topology/ — Topology validation
engine/           — Circuit execution (Build, Run, Walk, BatchWalk)
engine/handler/   — Handler types (transformer, extractor, renderer, hook)
engine/trace/     — Trace recording
mcp/              — MCP server (circuit_server, approval_tools)
fold/             — Compile YAML manifest → standalone binary
calibrate/        — Calibration framework (ScoreCard, metrics)
lint/              — Static analysis for circuit YAML
lsp/               — Language Server for circuit YAML
instruments/      — Framework instruments (oculus, gotools, llmfix)
simulate/sdlc/    — SDLC simulation
operator/         — Operator pattern (observe/reconcile/evaluate)
resource/         — Unified Resource API (16 kinds, KindRegistry)
prompt/           — Prompt types, Store, template enforcement
dispatch/         — MuxDispatcher, worker dispatch
budget/           — Cost tracking types
toolkit/          — Shared utilities (MatchEvaluator, etc.)
```

## DSL — Instruments are THE dispatch model

```yaml
# Declarative:
- name: scan
  instrument: oculus
  action: scan

# Imperative escape hatch:
- name: scan
  instrument: oculus
  command: "oculus scan --format=json"
```

- `handler_type` / `handler` are **DEAD** — killed in TSK-657 (82 files)
- `instrument` + `action` are canonical
- Instruments are compiled Go binaries or in-process functions (dispatch: go)
- Security: checksum pinning + output schema validation

## Naming Conventions

- **Instruments**: use "instrument" not "component" or "connector" (both banned)
- **Kinds**: PascalCase (Schematic, Board, Instrument, Scorecard, Scenario)
- **Engine vs schematics**: schematics provide config, engine executes. Never say a schematic "does work."
- **Circuits not DAGs**: "DAG" is banned. Origami executes circuit graphs.

## Go Conventions

- Go 1.25+
- golangci-lint enforced via pre-commit hook
- American English spelling
- Sentinel errors, slog structured logging
- Direct troupe/battery imports (no adapter layer)

## MCP Architecture

```
Claude Code → origami orchestrate (stdio, proxies + workers tool)
               → mediator (Docker :9000, routes circuit/signal)
                   → RCA (:9300) + GND (:9100)
               → spawns agent CLIs locally (workers tool)
```

Single MCP entry: `origami orchestrate --endpoint http://localhost:9000/mcp`

## Active Work

- **GOL-82**: Approval Gate — engine parks walkers at gated nodes, MCP approval tool
- **Instrument dispatch**: InstrumentNode + ExecDispatcher (TSK-660)
- **ORG-TSK-458**: Drain worker code from origami (lives in bugle/orchestrate now)

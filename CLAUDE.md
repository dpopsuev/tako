# Claude Code Instructions for Origami Development

## What is Origami

Origami is a circuit agent orchestration framework — YAML DAGs executed by AI agents over MCP. It implements the Bugle Protocol (served via mcp/circuit_server.go) and manages circuit execution, calibration, and fold codegen.

- Repo: github.com/dpopsuev/origami
- Scribe scope: origami
- Campaign: ORG-CMP-3 (active)

## Ecosystem Dependency Rules (JRC-SPC-2)

**CRITICAL: Origami imports Jericho, never the reverse. Origami never imports Djinn.**

- Origami -> Jericho (via agentport/ adapter). This is the ONLY Jericho import path.
- Origami NEVER imports djinn/
- Djinn communicates with Origami via Bugle Protocol, not Go imports
- dispatch/ types are being absorbed FROM Jericho into Origami (they are circuit-specific)

Dependency direction: `Origami -> Jericho <- Djinn` (lateral via Bugle Protocol)

## Key Packages

```
circuit/      — Circuit types (Node, Edge, Walker, DAG)
engine/       — Circuit execution engine (Run, Validate, Build)
mcp/          — MCP server (circuit_server serves the bugle tool)
fold/         — Compile YAML manifest to standalone binary
calibrate/    — Calibration framework (ScoreCard, metrics)
lint/         — Static analysis for circuit YAML
lsp/          — Language Server for circuit YAML
agentport/    — Hexagonal adapter to Jericho (THE ONE SEAM)
toolkit/      — Shared utilities (MatchEvaluator, etc.)
```

## Naming Conventions

- **Bugle Protocol verbs**: pull/push (not step/submit) — being migrated
- **Tool name**: registering as `bugle` with `circuit` alias for backward compat
- **Components**: use "component" not "connector" (banned term)
- **Engine vs schematics**: schematics provide config, engine executes. Never say a schematic "does work."

## Go Conventions

- Go 1.25+
- golangci-lint enforced via pre-commit hook
- American English spelling
- Sentinel errors, slog structured logging

# Origami DSL Keyword Registry

> 3 Kinds. 6 Axes. 30 Keywords.
> Adding a keyword is a permanent API commitment.
> If it's not listed here, it's not a keyword.
>
> Scribe: ORG-DOC-14 | Philosophy: ORG-DOC-13

## 3 Kinds (Electronics Metaphor)

| Full Name | Short (`kind:`) | File | Who writes it | Layer |
|-----------|-----------------|------|---------------|-------|
| CircuitSchematic | `schematic` | `circuits/*.yaml` | Circuit developers | 2 |
| CircuitComponent | `component` | `component.yaml` | Component developers | 2 |
| CircuitBoard | `board` | `origami.yaml` | Implementors | 3 |

## Axis 1: IDENTITY (4 keywords — all kinds)

| Keyword | Purpose | S | C | B |
|---------|---------|---|---|---|
| `kind` | schematic / component / board | ✓ | ✓ | ✓ |
| `name` | unique identifier | ✓ | ✓ | ✓ |
| `version` | semver | ✓ | ✓ | ✓ |
| `module` | Go import path | | ✓ | ✓ |

## Axis 2: TOPOLOGY (7 keywords — schematic only)

| Keyword | Purpose |
|---------|---------|
| `nodes` | node list |
| `edges` | edge list |
| `from` | source node |
| `to` | target node |
| `when` | condition expression |
| `start` | entry node |
| `done` | exit node |

## Axis 3: CONTRACTS (5 keywords — component only)

| Keyword | Purpose | Enforced types |
|---------|---------|----------------|
| `needs` | what it requires | — |
| `transports` | ingress sockets | Transport, Trigger (lint S40) |
| `sources` | data sockets | SourceReader, SourceCatalog (lint S41) |
| `storage` | persistence sockets | Driver (lint S42) |
| `gives` | what it provides | — |

## Axis 4: COMPOSITION (6 keywords)

| Keyword | Purpose | S | B |
|---------|---------|---|---|
| `import` | base schematic | ✓ | |
| `ports` | typed boundaries | ✓ | |
| `wiring` | port connections | ✓ | |
| `uses` | schematics + components | | ✓ |
| `bind` | socket → component | | ✓ |
| `domains` | data directories | | ✓ |

## Axis 5: BEHAVIOR (6 keywords)

| Keyword | Purpose | S | C | B |
|---------|---------|---|---|---|
| `handler` | type:name (e.g. `transformer:recall`) | ✓ | | |
| `approach` | methodical / exploratory / adversarial | ✓ | | |
| `before` | pre-node hooks | ✓ | | |
| `after` | post-node hooks | ✓ | | |
| `hooks` | Go SchematicHooks symbol | | ✓ | |
| `serve` | MCP / server config | | ✓ | ✓ |

## Axis 6: VALIDATION (2 keywords — schematic only)

| Keyword | Purpose |
|---------|---------|
| `calibration` | scorer contracts |
| `zones` | spatial grouping |

## Budget

| Axis | Keywords |
|------|----------|
| Identity | 4 |
| Topology | 7 |
| Contracts | 5 |
| Composition | 6 |
| Behavior | 6 |
| Validation | 2 |
| **Total** | **30** |

## Progressive Disclosure

| Week | New keywords | Audience | Cumulative |
|------|-------------|----------|------------|
| 1 | `kind` `name` `version` `module` `uses` `bind` `domains` `serve` | Board authors (Layer 3) | 8 |
| 2 | `needs` `transports` `sources` `storage` `gives` | Component developers | 13 |
| 3 | `nodes` `edges` `from` `to` `when` `start` `done` `handler` `approach` `before` `after` `hooks` | Schematic developers | 25 |
| 4 | `import` `ports` `wiring` `calibration` `zones` | Advanced composition | 30 |

## Keywords Killed (from 64 → 30)

| Killed | Replaced by |
|--------|-------------|
| `component` (as identity) | `kind: component` |
| `circuit` (as identity) | `kind: board` or `kind: schematic` |
| `namespace` | derived from name |
| `metadata` | `name` + `version` at top level |
| `factory` | absorbed into `gives: socket: Factory` |
| `satisfies` | `gives` |
| `requires` | `needs` |
| `sockets` | typed sections (`transports` / `sources` / `storage`) |
| `socket`, `type`, `option` | `name: Type` map syntax |
| `optional`, `schematic` | rare annotations, not keywords |
| `wire` | default instance, annotation when needed |
| `provides` | `hooks` symbol handles registration |
| `params`, `schemas`, `report`, `dispatch` | under `serve:` |
| `meta` | **deleted** — philosophy violation (ORG-DOC-13) |
| `schematics`, `connectors` | `uses` |
| `bindings` | `bind` under `uses` |
| `domain_serve` | `serve` |
| `assets`, `store_wiring` | under `serve` / `domains` |
| `id` (edge) | `from` + `to` is the identity |
| `handler_type` | merged into `handler: type:name` |

# Claude Code Instructions for Tako Development

## What is Tako

Tako is a Jidoka Agent Platform — DSAC (Domain Specific Agent Collective) mono-binaries assembled from Blueprints. Tangled is the Federated Agent Runtime (FAR). Tako is the application that runs on FAR.

- Repo: github.com/dpopsuev/origami (renaming to tako, GOL-175)
- Scribe scopes: `tako` (application), `tangled` (runtime engine), `mirage` (isolation)
- Campaign: ORG-CMP-24 (umbrella) — 6 child campaigns + CMP-44 (Tangle Reform)
- Architecture doc: ORG-DOC-44 (200+ sections, needs reconciliation)
- Package layout: ORG-DOC-58 (canonical, 2026-04-24)

## Ecosystem

- Tako → Tangled (FAR: connectivity, routing, state, auth, observability, execution)
- Tako → Mirage (Agent Space isolation: overlay, container, virtual backends)
- Tako → Oculus (code analysis instrument)
- Battery absorbed into origami/tool/ (GOL-102, done)
- Djinn patterns stolen, not imported (Terminal, Space, Envelope)

## Tangled — Federated Agent Runtime (FAR)

Tako is a Factory. Tangled is the Utilities — electricity, water, railway, security, inspectors. The factory doesn't build its own power plant, it plugs in. Tangled serves any factory, doesn't know what's manufactured inside. Tangled can run standalone.

5-level architecture aligned with ISA-95 Automation Pyramid:
- L4 Coordination (Tako): Director → Collectives hierarchy. HITL aggregation.
- L3 Services (Tako): Kanban, Andon, Discourse, Sleep. Stigmergic coordination.
- L2 Routing (Tangled): Switchboard (TangleD). Wire routing. Star topology.
- L1 Connectivity (Tangled): Tangle client/server. Embed/Connect. VersionGate.
- L0 Execution (Tangled): AXI. Mirage enclosure. Instrument execution. LLM calls. The physical process.

6 interface families (TNG-DOC-1):
- **AAI** — Agent Auth Interface (DEFINES trust: Identity, Capability, Audit)
- **ARI** — Agent Runtime Interface (exist: Probe, Lifecycle, Caster)
- **ANI** — Agent Network Interface (connect: Admission, Gate, Registry)
- **ASI** — Agent State Interface (persist: Stateful seam, StateStore, Config, Meter). Meter tracks whatever the utility provides — CPU/mem/IO AND LLM tokens when LLM is infrastructure.
- **AOI** — Agent Observability Interface (watch: Health events, Admin, OTel shape). Tangled emits raw signals, Tako interprets (Andon, OAE).
- **AXI** — Agent Execution Interface (execute: Executor, Sandbox, ResourceLimit, Policy) — L0, OPTIONAL, Workstation Organ only. Policy IS Mirage.Spec. Breaches emit events, Tako's Andon interprets.

AAI defines contracts. Other families enforce at their boundary. Defense-in-depth.

## Target Package Layout (DOC-58)

```
tako/
  ├─ fab/                     ← Fab Graph + Channel (stigmergy)
  │   ├─ node/                ← Stations (StationNode), submission contracts
  │   └─ edge/                ← Assertions, matches, gates
  ├─ instrument/              ← Contracts & composition primitives
  │   ├─ function/            ← Function interface, Result type
  │   └─ shell/               ← Shell contract (search/man/exec)
  ├─ contextual/              ← Current Context assembly (NOW) — leaf
  │   └─ block/               ← Block type, scopes, assembly strategy
  ├─ memory/                  ← Saved Memory (Perennial)
  │   ├─ node/                ← Knowledge/KnowledgeNode (What) — graph nodes
  │   ├─ edge/                ← Understanding (How/Why) — relationships
  │   └─ walk/                ← Wisdom (When) — traversal history
  ├─ artifact/                ← Artifact types (Envelope, versioning, claims) — leaf
  ├─ render/                  ← Blackboard + Panel/Event + Renderable contracts — leaf
  │                             Scene, Node, Renderable, Workspace, Viewport
  │                             Panel, Layout, Table, Status, List, Chart, Tree, Text, Action, Toolbar
  ├─ tangle/                  ← Tangle client port (interfaces Tako expects from Tangled) — leaf
  ├─ workstation/             ← Pre-configured environment (exists without agent)
  │   └─ builtin/             ← Station-level instruments (always present)
  │       ├─ envelope/        ← Artifact lifecycle (create/claim/push/release)
  │       └─ andon/           ← Station health state (pull/clear)
  ├─ agent/                   ← Agent runtime
  │   ├─ corpus/              ← Agent body — assembled from AAI.Capability blueprint
  │   ├─ organ/               ← Functional parts attached to the Corpus
  │   │   ├─ workstation/     ← Remote connection to Station instruments (Workers only)
  │   │   ├─ dialogue/        ← External Discourse (communicate)
  │   │   ├─ monologue/       ← Internal Discourse (focus, pin, Topics)
  │   │   ├─ kanban/          ← Board observer (read-only)
  │   │   ├─ andon/           ← Health observer (read-only)
  │   │   └─ blackboard/       ← Shared Blackboard reader (Avatar only)
  │   └─ runtime/             ← FSM + ReAct loop
  ├─ assemble/                ← DSL compiler (Blueprint → mono-binary)
  │   ├─ parser/              ← YAML parser for 6 kinds
  │   └─ validator/           ← Lint, schema refs, reachability
  ├─ catalog/                 ← Discovery
  │   ├─ instruments/         ← Instrument registry + resolution
  │   └─ blueprints/          ← Blueprint registry + versioning
  ├─ service/                 ← Agent Services (stigmergic + coordination)
  │   ├─ depo/                ← Artifact storage with Shelves (push/pull API, embed/connect modes)
  │   ├─ kanban/              ← Board projection (reads from Warehouse, read-only)
  │   ├─ andon/               ← Escalation engine (two-pull + takt time)
  │   ├─ discourse/           ← Shared primitives (board.forum.topic.thread.message)
  │   ├─ mailbox/             ← Async envelopes + HITL routing
  │   └─ sleep/               ← Memory lifecycle (drain Working → Saved, consolidate)
  ├─ vision/                  ← Tap management + adapter wiring
  │   └─ adapter/             ← Session-type bridges (render.Blackboard → ui/)
  │       ├─ headless/        ← Noop (tap closed)
  │       ├─ tui/             ← Bridges render.Blackboard → ui/term/
  │       └─ webui/           ← Bridges render.Blackboard → ui/web/
  ├─ ui/                      ← UI implementations
  │   ├─ term/                ← Terminal UI (widget/, layout/, design/)
  │   └─ web/                 ← Web UI (stub)
  ├─ cmd/tako/                ← Entry point (boots TUI)
  ├─ inspector/               ← Core: OAE scoring, quality inspection (production + rehearsal)
  ├─ rehearsal/               ← Scaffolding: same cranes internally and externally (mock/, fixtures/)
  └─ blueprints/
      └─ autoassembler/       ← The first Blueprint (Self-Assembly)
```

Depth rules: depth 1 = domain boundary, depth 2 = facet, depth 3 = implementation detail.
Leaf packages (no deps, everyone imports): contextual/, artifact/, render/, tangle/.

## Agent Space — The Zoo Model (TAK-DOC-1)

Agent Space is a partitioned, sandboxed region of User Space managed by Tangled via Mirage. Not a sandbox illusion — real partitioned environment with access control.

- Agent Space > Sandbox. Sandbox is one implementation strategy within Agent Space.
- Mirage backends: Overlay (fs), Container (ns+cgroup), Virtual (Kata VM), Stub (testing)
- Three nested lifecycles: Tangled > Agent Space > Workstation
- AXI controls the gate. Workers have AXI. Director/Foreman/Avatar do not.

## Collective Topology (SPC-130)

```
Human → Avatar (co-pilot, human's presence in Agent Space)
           ↕ Mailbox (Blackboard pattern)
      Switchboard (TangleD — routes, star topology)
           ↕
  Director ←→ Collective A ←→ Collective B
```

- **Director** = always present (Root Agent). Hierarchical authority over Collectives.
- **Foreman** = Collective facade. Manages internal Workers. Responds to Andon pulls.
- **Workers** = Execution. Only entity with AXI (instrument access).
- **Avatar** = Human proxy. Co-pilot. Eyes + voice, not hands. No AXI.
- Switchboard = routing hub (L2). Director = authority (L4). Different layers.
- Operator talks to Director only. Foreman never talks to operator.

Persona → Uniform → Organs (corpus blueprint from AAI.Capability):
- Worker: Monologue, Dialogue, Kanban, Andon, Workstation (has remote Station access)
- Foreman: Monologue, Dialogue, Kanban, Andon (observe + communicate, no Workstation)
- Director: Monologue, Dialogue, Kanban (Fab-level view)
- Avatar: Monologue, Dialogue, Kanban, Andon, Blackboard (human proxy)

Agents don't know about interface families (AAI/ARI/ANI/ASI/AOI/AXI) — that's Tangled plumbing.
Uniforms are declarative (defined in Fab YAML). Personas are well-known defaults.

## Key Concepts

- **Fab Graph** — production line (fab = fabrication plant). StationNodes, edges, artifacts flow through. Kanban projects from it.
- **Stigmergy** — coordination by doing. Services hook into manufactory/workstation/instruments. Kanban + Andon are read-only projections. Agents never write to boards.
- **Andon** — health signaling with 4 levels (Agent, Workstation, Station, Fab). Two-pull protocol: Yellow = Foreman responds, Red = escalates. CORDON = death spiral circuit breaker (Jidoka — stop the line). Only escalates to HITL on Red timeout, not by default. (SPC-132).
- **Three Blackboards** — same Blackboard pattern (shared knowledge structure, producers post, consumers subscribe), three domains: render.Blackboard (UI panels), service.Depo (work artifacts), discourse.Board (messages).
- **Depo** — Blackboard for artifact Pub-Sub. Push/Pull API — same interface internal or external. Each Station gets a Shelf. Workstation reads/writes to its Shelf. Embed mode = in-memory. Connect mode = external depod service. Agent state and Artifact state have separate persistence paths — agents crash, artifacts survive.
- **Shelf** — named location within a Depo. Push(envelope) = Blackboard.Post. Pull(agentID). Watch() = Blackboard.Subscribe(). Station Shelf, Intake Shelf, Output Shelf, HITL Shelf.
- **Kanban** — read-only projection of Depo Shelves. Kanban column = Shelf. Kanban card = Envelope. No data store — reads Depo state. Toyota mirror pattern (SPC-131).
- **Discourse** — shared primitives (board.forum.topic.thread.message). Monologue = internal scope (focus, pin). Dialogue = external scope (communicate).
- **Artifact** — the only type. Everything is an Artifact (Relic Protocol Node). Work artifacts on Depo Shelves. Memory artifacts in Monologue. Knowledge artifacts in Reliquary.
- **Relic** — NOT a type. A certification label (`certified:human`) stamped by humans only (HITL). Agents cannot self-certify. Certified artifacts get an anchor weight — gravity in the graph. High anchor = hard to move, attracts neighbors. Low anchor = drifts, evictable.
- **Dolt** — the artifact backend. One Dolt instance per Fab (embed first, self-contained). Depo Shelves, Monologue Topics, Dialogue Letters, Knowledge Mesh — all stored in Dolt. Git semantics: DOLT_COMMIT (drain), DOLT_BRANCH (agent session), DOLT_MERGE (collective), DOLT_TAG (certified:human). Embed: dolthub/driver (in-process). Shared Dolt across Fabs deferred until tensions emerge.- **Memory chain** — LLM (Instinct) ←→ Reactivity ←→ Monologue ←→ Recollection. Recollection queries Reliquary, pulls Artifacts into Monologue, forms Molecules (sub-graphs within Topic).
- **Memory tiers** — STM/Working (Monologue) = live context, per-session, Topics with Molecules. Depo Shelves = work artifacts on production line. LTM (Reliquary) = all artifacts (certified + draft), version-controlled mesh per agent, merged into collective mesh.
- **Two queries** — Knowledge Query (Reliquary, certified:human only): "what do I know?" Experience Query (Monologue): "what happened this session?"
- **Anchor weight** — gravity in the LTM graph. High anchor = gravity well, neighbors cluster, hard to evict. Low anchor = drifts, evictable. Decays on staleness. Set by human at certification.
- **Corpus** — the agent's body, assembled from AAI.Capability blueprint. Collection of Organs. Tangled builds the Corpus, agent never self-assembles.
- **Organ** — a functional part attached to the Corpus. Monologue, Dialogue, Kanban, Andon, Workstation, Blackboard. The Uniform declares which Organs attach.
- **Workstation (Organ)** — remote connection to a Station's instruments. The Station has the workbench, the agent has the remote control limb. Default: one agent per station, fixed assignment (Ford alignment). Workers don't roam. If a station is overloaded, add workers there, don't move them from elsewhere.
- **Station knowledge** — three layers at the station, none in the instrument. Prompt templates = Ford's jigs (guide the tool). Contracts = Ford's fixtures (hold the output in shape). Instrument cache = hash-based memoization (same input → cache hit). Semantic knowledge comes from the agent's Mesh via Recollection, not from the station or instrument.
- **Instrument cache** — hash-based only. Input signature → cached result. KISS. No Mesh, no vector search, no semantics at instrument level. Agents and instruments benefit from different things.
- **Event-driven Needs** — ReActivity is event-driven. Agents subscribe to Blackboard services (Depo, Andon, Kanban, Discourse). Events become Needs. Needs start Molecules. The Three Blackboards ARE the delivery belt. No separate trigger mechanism.
- **Ford divergence: Memory Disk** — Ford moved knowledge from workers into jigs because brains can't be cloned. We have both: station knowledge (jigs/fixtures/cache) AND agent knowledge (Mesh). Agent Mesh is a disk: serialize, clone, merge, fork via Dolt. sleep/ drains agent Mesh into collective Mesh (DOLT_MERGE). No knowledge lost. Agents are replaceable muscle WITH transferable memory.
- **Avatar** — human's co-pilot in Agent Space. Renders scene (compose UI). Acts on behalf (API proxy + A2A delegation). IS the vision tap. Persists on disconnect (tmux model). TAK-SPC-1.
- **render.Blackboard** — shared knowledge structure (Blackboard architecture, Hayes-Roth 1985). Sub-systems post Panels (Fab Map, Kanban, Andon, etc.). Avatar reads the Blackboard and composes a view. Not DOM, not Canvas — a Blackboard.
- **ReActivity** — NOT an FSM. A reaction engine (SPC-117). Typed artifacts (Intent, Assessment, Plan, Execution, Retrospection) bond into Molecules. Agent is catalyst — reads open bonds, produces Atoms, Assert validates bonds. Back-pressure reverses flow when bonds unsatisfied. Wish seals the Molecule, closes the Topic. Prior art: CHAM (Berry & Boudol 1992), BDI (Rao & Georgeff 1995), Colored Petri Nets.
- **Molecule** — compound of bonded Atoms within a Monologue Topic. Mass = accumulated artifacts. Depth = how far into the reaction chain. Fission = decomposition into sub-Molecules (breadth). Sealed by Wish.
- **Cynefin Classification** — Molecule composition determines Cynefin domain (Dave Snowden). High Recollection mass = Clear (cheap model, loose gates). High Assessment mass = Complex (Opus, strict gates). All unknowns = Chaotic (HITL). Drives Caster.Pick, Assert sensitivity, and HITL threshold automatically.
- **Drain/Hydrate** — universal lifecycle primitive. Same mechanism for crash recovery, self-update, and federation (Embed → Switchboard live migration). StateStore is the migration wire.
- **AuditEntry** — universal WHO/WHAT/WHICH/WHEN/WHERE/RESULT record. One struct, all 6 families. Append-only. K8s audit level pattern (TNG-SPC-1).

## DSL — 3 Top-Level Kinds

Complex, Fab, Rehearsal. PascalCase (K8s convention).
Blueprint is dead — absorbed into Complex + Fab.
Station, Contract, Instrument are inline sections within Fab, not separate kinds.
Complex is thin (refs + wiring + topology). Fab has the substance. Rehearsal validates.

## Naming Conventions

- **Instruments**: use "instrument" not "component" or "connector" (both banned)
- **Fab not activity/circuit/manufactory**: the production line graph (fab = fabrication plant)
- **StationNode not Node**: disambiguate from memory.KnowledgeNode
- **DAG is banned**: use "fab graph"
- **No Ouroboros**: use "Self-Assembly" or "Autoassembler"
- **No Cortex**: use "contextual" or "memory"
- **No Battery (package)**: absorbed into tako/instrument/
- **No FSM for ReActivity**: it's a reaction engine, not a state machine. Molecule drives the agent.
- **No Canvas**: use "Blackboard" (render.Blackboard). Canvas is dead terminology.
- **No compose/fold**: dead. CLI is "tako assemble". Instrument is "tako.assemble". Same API.
- **No janitor**: use "sleep" (service/sleep/)
- **No DOM, no Scene, no Canvas**: use "Blackboard" (render.Blackboard — shared knowledge structure for Operator + Avatar)
- **No sandbox (as concept)**: use "Agent Space". Sandbox is one implementation strategy.
- **No Terminal/TerminalMux**: use "Corpus" (agent body) and "Organ" (functional part). Workstation is an Organ, not a separate system.
- **No FAR**: use "FAR" (Federated Agent Runtime). Three letters, not four.
- **Persona not role**: Worker/Foreman/Director/Avatar are Personas. Uniforms are the permission sets they wear. Organs are the limbs they get.
- **Dolt is the backend**: services talk to Dolt directly through their own interfaces. Reliquary deferred until an interface layer proves necessary.
- **Artifact vs Relic**: same type. Relic = `certified:human` label + anchor weight. NOT a type distinction — a quality gate.
- **Switchboard**: TangleD is the Switchboard (routing hub). Not "hub."

## Go Conventions

- Go 1.25+
- golangci-lint enforced via pre-commit hook
- American English spelling
- Sentinel errors, slog structured logging
- OTel for all observability (Tangled owns contracts, Tako complies)
- Tangle v0.16.0 (identity→visual, execution→providers, Broker→Caster)

## Active Work

### Scribe Scopes
- `tako` — application (campaigns, goals, specs, docs, tasks)
- `tangled` — runtime engine (CMP-44 Tangle Reform, interface families)
- `mirage` — isolation library (Agent Space enclosures)
- `origami` — archived (historical record, drained)
- `troupe` — partially migrated (needs assessment)

### Campaigns
- **CMP-24** (tako): umbrella — 6 child campaigns
- **CMP-35 Foundations** (tako): vertical slice — Assessment → Skeleton → Wire → Cleanup
- **CMP-36 Single Agent Depth** (tako): replace stubs with real implementations
- **CMP-37 Collective** (tako): multi-agent topology
- **CMP-38 Composition** (tako): full TUI + DSL + instruments + Autoassembler
- **CMP-39 Autoassembler** (tako): Self-Hosting & Self-Assembly
- **CMP-40 Federation** (tako): inter-DSAC Director coordination
- **CMP-44 Tangle Reform** (tangled): Troupe → Tangle evolution (8 goals)

### Key Specs
- SPC-129: Autoassembler Collective (5 agents, first Blueprint)
- SPC-130: Director & Topology (Switchboard star, hierarchical authority)
- SPC-131: Kanban (stigmergic projection, Toyota mirror)
- SPC-132: Andon (two-pull escalation = HITL)
- SPC-134: DSAC Assembly (mono-binary, BusyBox pattern)
- TAK-SPC-1: Avatar (human proxy, co-pilot)
- TNG-SPC-1: AuditEntry (universal audit record)

### Key Docs
- DOC-44: DSAC Architecture (200+ sections, needs reconciliation)
- DOC-58: Tako Target Package Layout (canonical)
- TAK-DOC-1: Agent Space — The Zoo Model
- TAK-DOC-2: Render Scene — Self-Composing UI
- TNG-DOC-1: Tangled Interface Families (AAI/ARI/ANI/ASI/AOI/AXI)
- TAK-ADR-1: FAR 4-Layer Architecture
- TAK-ADR-2: Cloud-Native Alignment (CRI/Agent Sandbox/CRIU)

### First Tasks
- GOL-175: Rename Origami → Tako (CMP-35)
- GOL-173: Rename Troupe → Tangle (CMP-44)

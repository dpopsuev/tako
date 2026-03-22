// Package framework is the core of the Origami agentic circuit framework.
//
// This package provides graph-based circuit orchestration: define circuits
// in YAML, build typed node/edge graphs, walk them with AI-powered agents,
// and observe execution with hooks, renderers, and narrators.
//
// # Reading guide
//
// The package is organized into five layers. Read them in order for
// a progressive understanding; start with Core, skip to Execution
// if you just want to run circuits.
//
// ## Layer 1 — Core Primitives
//
// The minimal graph model. Everything else builds on these types.
//
//   - [Node]         — processing stage (node.go)
//   - [DelegateNode] — node that generates and walks a sub-circuit (delegate.go)
//   - [Edge]         — conditional connection between nodes (edge.go)
//   - [ExpressionEdge] — edge with CEL expression evaluation (expression_edge.go)
//   - [Graph]        — directed graph of nodes and edges (graph.go)
//   - [Walker]       — agent traversing a graph (walker.go)
//   - [Element]      — named archetype for walker behavior (element.go → element/)
//
// ## Layer 2 — DSL & Build
//
// Circuit definition and construction from YAML.
//
//   - [CircuitDef], [LoadCircuit] — parse circuit YAML into a typed definition (dsl.go)
//   - [Component]    — reusable circuit building block with FQCN resolution (component.go)
//   - [BuildWalker]  — construct a Walker from a WalkerDef (walker_build.go)
//   - [ResolveVars]  — template variable substitution in circuit definitions (resolve.go)
//   - [ArtifactSchema] — JSON Schema validation for node outputs (schema.go)
//   - [Render]       — circuit definition rendering (render.go)
//   - [CircuitVars]  — circuit-level variable definitions (vars.go)
//
// ## Layer 3 — Processing & Support
//
// Pluggable processors that transform, extract, and observe walker steps.
//
//   - [Extractor]    — pull structured data from LLM output (extractor.go)
//   - [Transformer]  — modify walker context between steps (transformer.go)
//   - [Hook]         — pre/post step callbacks (hook.go)
//   - [Renderer]     — format walker state for display (renderer.go)
//   - [Observer]     — receive walker events (observer.go)
//   - [Narrator]     — human-readable step narration (narrate.go)
//   - [Vocabulary]   — domain term definitions for prompts (vocabulary.go)
//   - [Capture]      — output capture utilities (capture.go)
//   - [Identity]     — walker identity (persona + element + role) (identity.go)
//   - [IsDeterministic] — determinism analysis for nodes/circuits (determinism.go)
//   - models/        — LLM model registry (models/registry.go)
//   - [Finding], [FindingSeverity]       — typed enforcer observations (core/ + finding/)
//   - [FindingCollector], [InMemoryFindingCollector] — finding accumulation (finding/)
//   - [FindingRouter], [RouteRule]       — severity+domain routing to authorities (finding/)
//   - [VetoHook]     — after-hook that vetoes on FindingError (finding/)
//   - [Errors]       — sentinel errors (errors.go)
//   - [Extractors]   — built-in extractor implementations (extractors.go)
//
// ## Layer 4 — Execution
//
// How circuits run: scheduling, caching, fan-out, checkpointing.
//
//   - [Run]          — top-level entry point: load circuit + walk (run.go) ← START HERE
//   - [Runner]       — configurable circuit executor (runner.go)
//   - [Scheduler]    — step scheduling and ordering (scheduler.go)
//   - [Checkpoint], [Checkpointer] — durable execution with save/resume (checkpoint.go)
//   - [Cache], [InMemoryCache]     — node result caching with TTL (cache.go)
//   - [Memory], [MemoryStore]      — persistent walker memory (memory.go)
//   - [BatchWalk]    — walk a circuit across multiple inputs (batch_walk.go)
//   - [Operator], [RunOperator]    — reconciliation loop: observe → evaluate → reconcile → walk (operator.go)
//   - [CircuitContainer], [InMemoryContainer] — circuit lifecycle management (operator.go)
//   - [FanOut]       — parallel node execution (fanout.go)
//   - [Team]         — multi-walker collaborative execution (team.go)
//   - WithThermalBudget — cumulative latency budget enforcement (thermal.go)
//   - [RunWithEnforcer] — parallel work + enforcer circuit execution (finding_parallel.go)
//
// ## Layer 5 — Circuit Features (sub-packages)
//
// Optional capabilities extracted into their own packages. None are
// required for basic usage; import only what you need.
//
//   - finding/   — finding collector, router, veto hook, veto artifact
//   - element/   — behavioral archetypes (Approach, Element, SpeedClass, traits)
//   - models/    — foundation LLM model registry
//   - topology/  — circuit topology validation (cascade, fan-out, fan-in, etc.)
//   - persona/   — 8 perennial agent identities; import for [PersonaResolver] registration
//   - mask/      — detachable node middleware (Mask of Recall, Forge, etc.)
//   - dialectic/ — thesis/antithesis/synthesis debate, evidence gaps, CMRR
//   - cycle/     — generative and destructive element interaction rules
//
// # Quick start
//
// To run a circuit from YAML:
//
//	err := framework.Run(ctx, "path/to/circuit.yaml", input,
//	    framework.WithRegistries(registries),
//	)
//
// To load and inspect a circuit definition:
//
//	def, err := framework.LoadCircuit(yamlBytes)
package framework

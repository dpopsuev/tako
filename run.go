package framework

// Category: Execution

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// RunOption configures a Run invocation.
type RunOption func(*runConfig)

type runConfig struct {
	transformers   TransformerRegistry
	hooks          HookRegistry
	extractors     ExtractorRegistry
	nodes          NodeRegistry
	edges          EdgeFactory
	components     ComponentLoader
	overrides      map[string]any
	walker         Walker
	team           *Team
	observer       WalkObserver
	logger         *slog.Logger
	memory         MemoryStore
	nodeCache      NodeCache
	checkpointer   Checkpointer
	resumeID       string
	resumeInput       any
	thermalBudget     *thermalConfig
	offsetPreamble    string
}

// WithTransformers registers transformers for the run.
func WithTransformers(reg TransformerRegistry) RunOption {
	return func(c *runConfig) { c.transformers = reg }
}

// WithHooks registers hooks for the run.
func WithHooks(reg HookRegistry) RunOption {
	return func(c *runConfig) { c.hooks = reg }
}

// WithExtractors registers extractors for the run.
func WithExtractors(reg ExtractorRegistry) RunOption {
	return func(c *runConfig) { c.extractors = reg }
}

// WithNodes registers node factories for the run.
func WithNodes(reg NodeRegistry) RunOption {
	return func(c *runConfig) { c.nodes = reg }
}

// WithEdges registers edge factories for the run.
func WithEdges(reg EdgeFactory) RunOption {
	return func(c *runConfig) { c.edges = reg }
}

// WithComponents registers a component loader for the run. When the circuit
// YAML contains imports: [...], the loader is called for each import name
// and the resulting components are merged into the registries.
func WithComponents(loader ComponentLoader) RunOption {
	return func(c *runConfig) { c.components = loader }
}

// WithOverrides sets variable overrides (equivalent to --set key=value).
func WithOverrides(overrides map[string]any) RunOption {
	return func(c *runConfig) { c.overrides = overrides }
}

// WithWalker sets a custom Walker. If nil, ProcessWalker is used.
func WithWalker(w Walker) RunOption {
	return func(c *runConfig) { c.walker = w }
}

// WithTeam enables multi-walker team execution. When set, Run() dispatches
// to graph.WalkTeam instead of graph.Walk. WithWalker is ignored when
// WithTeam is set.
func WithTeam(team *Team) RunOption {
	return func(c *runConfig) { c.team = team }
}

// WithRunObserver attaches a walk observer for the run.
func WithRunObserver(obs WalkObserver) RunOption {
	return func(c *runConfig) { c.observer = obs }
}

// WithLogger sets the logger for the run.
func WithLogger(l *slog.Logger) RunOption {
	return func(c *runConfig) { c.logger = l }
}

// WithMemory attaches a MemoryStore for cross-walk persistence.
// The store is injected into each walker's context as "memory" so nodes
// can read/write persistent state scoped by walker identity.
func WithMemory(store MemoryStore) RunOption {
	return func(c *runConfig) { c.memory = store }
}

// WithTaggedMemory wraps a MemoryStore so that every SetNS call during the
// walk automatically attaches the given tags. This is useful for labeling
// all memories produced during a specific calibration run, scenario, or walk.
func WithTaggedMemory(store MemoryStore, tags ...string) RunOption {
	return func(c *runConfig) {
		c.memory = &taggedMemoryStore{Inner: store, Tags: tags}
	}
}

// WithNodeCache enables node-level caching. When a node has a CacheDef,
// the runner checks the cache before processing. On cache hit, the cached
// artifact is returned and eventNodeCacheHit is emitted.
func WithNodeCache(cache NodeCache) RunOption {
	return func(c *runConfig) { c.nodeCache = cache }
}

// WithCheckpointer enables auto-checkpointing: the runner saves walker
// state after each successful node, and removes the checkpoint on
// successful walk completion.
func WithCheckpointer(cp Checkpointer) RunOption {
	return func(c *runConfig) { c.checkpointer = cp }
}

// WithResume loads a previously saved checkpoint and continues the walk
// from the last checkpointed node instead of the circuit's start node.
// Requires WithCheckpointer to be set.
func WithResume(walkerID string) RunOption {
	return func(c *runConfig) { c.resumeID = walkerID }
}

// WithResumeInput loads a checkpoint and injects input into the walker's
// context as "resume_input" before continuing. Used for HITL flows where
// a human provides data to resume an interrupted walk.
func WithResumeInput(walkerID string, input any) RunOption {
	return func(c *runConfig) {
		c.resumeID = walkerID
		c.resumeInput = input
	}
}

// WithOffsetCompensation prepends a corrective preamble to each walker's
// PromptPreamble before the walk starts. Generate the preamble via
// ouroboros.OffsetCompensator.Compensate() and pass the result here.
// This avoids an import cycle between the root package and ouroboros.
func WithOffsetCompensation(preamble string) RunOption {
	return func(c *runConfig) { c.offsetPreamble = preamble }
}

// Run loads a circuit YAML, builds a graph, and walks it.
// This is the primary Go API for executing Origami circuits.
//
//	err := framework.Run(ctx, "circuits/rca.yaml", input,
//	    framework.WithTransformers(reg),
//	    framework.WithHooks(hooks),
//	    framework.WithOverrides(map[string]any{"recall_hit": 0.9}),
//	)
func Run(ctx context.Context, circuitPath string, input any, opts ...RunOption) error {
	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	data, err := os.ReadFile(circuitPath)
	if err != nil {
		return fmt.Errorf("read circuit %s: %w", circuitPath, err)
	}

	def, err := LoadCircuit(data)
	if err != nil {
		return fmt.Errorf("parse circuit %s: %w", circuitPath, err)
	}

	if len(cfg.overrides) > 0 {
		def.Vars = MergeVars(def.Vars, cfg.overrides)
	}

	reg := GraphRegistries{
		Nodes:        cfg.nodes,
		Edges:        cfg.edges,
		Extractors:   cfg.extractors,
		Transformers: cfg.transformers,
		Hooks:        cfg.hooks,
		Components:   cfg.components,
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		return fmt.Errorf("build runner: %w", err)
	}
	runner.Logger = cfg.logger

	obs := cfg.observer
	if cfg.thermalBudget != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		obs = &thermalObserver{
			inner:   obs,
			warning: cfg.thermalBudget.warning,
			ceiling: cfg.thermalBudget.ceiling,
			cancel:  cancel,
		}
	}

	if obs != nil {
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			dg.observer = obs
		}
	}

	if cfg.team != nil {
		for _, w := range cfg.team.Walkers {
			if input != nil {
				w.State().Context["input"] = input
			}
			if cfg.memory != nil {
				w.State().Context["memory"] = cfg.memory
			}
			if cfg.offsetPreamble != "" {
				applyOffsetPreamble(w, cfg.offsetPreamble)
			}
		}
		return runner.Graph.WalkTeam(ctx, cfg.team, def.Start)
	}

	walker := cfg.walker
	if walker == nil {
		walker = NewProcessWalker("run")
	}

	if cfg.offsetPreamble != "" {
		applyOffsetPreamble(walker, cfg.offsetPreamble)
	}

	startNode := def.Start

	if cfg.checkpointer != nil && cfg.resumeID != "" {
		loaded, loadErr := cfg.checkpointer.Load(cfg.resumeID)
		if loadErr != nil {
			return fmt.Errorf("load checkpoint %s: %w", cfg.resumeID, loadErr)
		}
		if loaded != nil {
			*walker.State() = *loaded
			startNode = loaded.CurrentNode
			if cfg.observer != nil {
				emitEvent(cfg.observer, WalkEvent{Type: EventWalkResumed, Node: startNode, Walker: walker.Identity().PersonaName})
			}
		}
	}

	if cfg.resumeInput != nil {
		walker.State().Context["resume_input"] = cfg.resumeInput
	}

	if input != nil {
		walker.State().Context["input"] = input
	}
	if cfg.memory != nil {
		walker.State().Context["memory"] = cfg.memory
	}

	if cfg.checkpointer != nil {
		walker = &checkpointingWalker{inner: walker, cp: cfg.checkpointer}
	}

	err = runner.Walk(ctx, walker, startNode)

	if err == walkInterrupted {
		return nil
	}

	if err == nil && cfg.checkpointer != nil {
		cpID := walker.State().ID
		if rmErr := cfg.checkpointer.Remove(cpID); rmErr != nil {
			slog.Warn("failed to remove checkpoint after successful walk",
				slog.String("walker_id", cpID),
				slog.String("error", rmErr.Error()),
			)
		}
	}
	return err
}

// Validate loads and validates a circuit YAML without executing it.
// Checks: YAML syntax, referential integrity, expression compilation,
// transformer resolution, hook resolution.
func Validate(circuitPath string, opts ...RunOption) error {
	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	data, err := os.ReadFile(circuitPath)
	if err != nil {
		return fmt.Errorf("read circuit %s: %w", circuitPath, err)
	}

	def, err := LoadCircuit(data)
	if err != nil {
		return fmt.Errorf("parse circuit %s: %w", circuitPath, err)
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("validate circuit: %w", err)
	}

	reg := GraphRegistries{
		Nodes:        cfg.nodes,
		Edges:        cfg.edges,
		Extractors:   cfg.extractors,
		Transformers: cfg.transformers,
		Hooks:        cfg.hooks,
		Components:   cfg.components,
	}

	hasRegistries := reg.Nodes != nil || reg.Edges != nil || reg.Extractors != nil || reg.Transformers != nil || reg.Hooks != nil
	if hasRegistries {
		if _, err := BuildGraph(def, reg); err != nil {
			return fmt.Errorf("build graph (dry run): %w", err)
		}
		for _, nd := range def.Nodes {
			for _, hookName := range nd.After {
				if reg.Hooks != nil {
					if _, hErr := reg.Hooks.Get(hookName); hErr != nil {
						return fmt.Errorf("node %q: hook %q: %w", nd.Name, hookName, hErr)
					}
				}
			}
		}
	}

	return nil
}

// applyOffsetPreamble appends a corrective preamble to a walker's
// PromptPreamble via SetIdentity. This ensures the preamble lands
// on the typed AgentIdentity field where prompt rendering consumes it.
func applyOffsetPreamble(w Walker, offset string) {
	id := w.Identity()
	if id.PromptPreamble == "" {
		id.PromptPreamble = offset
	} else {
		id.PromptPreamble = id.PromptPreamble + "\n\n" + offset
	}
	w.SetIdentity(id)
}

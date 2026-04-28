package engine

// Category: Execution

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/dpopsuev/tako/circuit"
)

// RunOption configures a Run invocation.
type RunOption func(*runConfig)

type runConfig struct {
	instruments    InstrumentRegistry
	hooks          HookRegistry
	extractors     ExtractorRegistry
	nodes          NodeRegistry
	edges          EdgeFactory
	manifests      ManifestRegistry
	components     ComponentLoader
	overrides      map[string]any
	walker         circuit.Walker
	observer       circuit.WalkObserver
	logger         *slog.Logger
	memory         circuit.MemoryStore
	nodeCache      circuit.NodeCache
	checkpointer   circuit.Checkpointer
	resumeID       string
	resumeInput    any
	thermalBudget  *thermalConfig
	offsetPreamble string
	safeOpts       []circuit.SafeHandlerOption
	useSafe        bool
	tune           bool
	hub            Hub
}

// WithInstruments registers in-process instrument handlers for the run.
func WithInstruments(reg InstrumentRegistry) RunOption {
	return func(c *runConfig) { c.instruments = reg }
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

// WithManifests registers instrument manifests for the run.
func WithManifests(reg ManifestRegistry) RunOption {
	return func(c *runConfig) { c.manifests = reg }
}

// WithTune enables preflight instrument tuning before the walk starts.
// Each instrument's tune command is executed; the run fails fast if any
// instrument is not available.
func WithTune() RunOption {
	return func(c *runConfig) { c.tune = true }
}

// WithRunHub attaches a pre-built Hub to the run. The hub is wired into
// the graph so tools rotate on each node entry.
func WithRunHub(hub Hub) RunOption {
	return func(c *runConfig) { c.hub = hub }
}

// WithComponents registers a component loader for the run.
func WithComponents(loader ComponentLoader) RunOption {
	return func(c *runConfig) { c.components = loader }
}

// WithOverrides sets variable overrides (equivalent to --set key=value).
func WithOverrides(overrides map[string]any) RunOption {
	return func(c *runConfig) { c.overrides = overrides }
}

// WithWalker sets a custom Walker. If nil, ProcessWalker is used.
func WithWalker(w circuit.Walker) RunOption {
	return func(c *runConfig) { c.walker = w }
}

// WithCollective enables multi-walker execution via a CollectiveWalker.
// The selector picks which walker handles each node.
func WithCollective(walkers []circuit.Walker, selector WalkerSelector, opts ...CollectiveWalkerOption) RunOption {
	return func(c *runConfig) {
		c.walker = NewCollectiveWalker(walkers, selector, opts...)
	}
}

// WithRunObserver attaches a walk observer for the run.
func WithRunObserver(obs circuit.WalkObserver) RunOption {
	return func(c *runConfig) { c.observer = obs }
}

// WithLogger sets the logger for the run.
func WithLogger(l *slog.Logger) RunOption {
	return func(c *runConfig) { c.logger = l }
}

// WithSafeHandler wraps the run's logger with a SafeHandler for truncation
// and sensitive data redaction. Applied after logger resolution.
func WithSafeHandler(opts ...circuit.SafeHandlerOption) RunOption {
	return func(c *runConfig) {
		c.safeOpts = opts
		c.useSafe = true
	}
}

// WithMemory attaches a MemoryStore for cross-walk persistence.
func WithMemory(store circuit.MemoryStore) RunOption {
	return func(c *runConfig) { c.memory = store }
}

// WithTaggedMemory wraps a MemoryStore so that every SetNS call during the
// walk automatically attaches the given tags.
func WithTaggedMemory(store circuit.MemoryStore, tags ...string) RunOption {
	return func(c *runConfig) {
		c.memory = &TaggedMemoryStore{Inner: store, Tags: tags}
	}
}

// WithNodeCache enables node-level caching.
func WithNodeCache(cache circuit.NodeCache) RunOption {
	return func(c *runConfig) { c.nodeCache = cache }
}

// WithCheckpointer enables auto-checkpointing.
func WithCheckpointer(cp circuit.Checkpointer) RunOption {
	return func(c *runConfig) { c.checkpointer = cp }
}

// WithResume loads a previously saved checkpoint and continues the walk.
func WithResume(walkerID string) RunOption {
	return func(c *runConfig) { c.resumeID = walkerID }
}

// WithResumeInput loads a checkpoint and injects input into the walker's
// context as "resume_input" before continuing.
func WithResumeInput(walkerID string, input any) RunOption {
	return func(c *runConfig) {
		c.resumeID = walkerID
		c.resumeInput = input
	}
}

// WithOffsetCompensation is a no-op — PromptPreamble was removed
// in the AgentIdentity simplification. Kept for API compatibility.
func WithOffsetCompensation(_ string) RunOption {
	return func(_ *runConfig) {}
}

// Run loads a circuit YAML, builds a graph, and walks it.
// This is the primary Go API for executing Tako circuits.
//
//nolint:gocyclo,funlen // top-level orchestrator applies all RunOption combinations
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

	if len(def.Vars) > 0 {
		resolved, err := circuit.ResolveSecrets(def.Vars)
		if err != nil {
			return fmt.Errorf("resolve secrets: %w", err)
		}
		def.Vars = resolved
	}

	reg := &GraphRegistries{
		Nodes:       cfg.nodes,
		Edges:       cfg.edges,
		Extractors:  cfg.extractors,
		Instruments: cfg.instruments,
		Hooks:       cfg.hooks,
		Manifests:   cfg.manifests,
		Components:  cfg.components,
	}

	if cfg.tune && len(cfg.manifests) > 0 {
		if err := TuneAll(ctx, cfg.manifests, reg.InstrumentDir); err != nil {
			return fmt.Errorf("preflight tune: %w", err)
		}
	}

	runner, err := NewRunnerWith(def, reg)
	if err != nil {
		return fmt.Errorf("build runner: %w", err)
	}

	// Default logger: never nil, never silent.
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	if cfg.useSafe {
		cfg.logger = slog.New(circuit.NewSafeHandler(cfg.logger.Handler(), cfg.safeOpts...))
	}
	runner.Logger = cfg.logger

	// Auto-attach LogObserver: free structured walk logs for consumers.
	logObs := NewLogObserver(cfg.logger)
	obs := cfg.observer
	if obs != nil {
		obs = circuit.MultiObserver{obs, logObs}
	} else {
		obs = logObs
	}
	// Thermal budget: default 5min warning, 15min ceiling.
	// Override via WithThermalBudget. Always active — every circuit
	// gets runaway protection for free.
	tb := cfg.thermalBudget
	if tb == nil {
		tb = &thermalConfig{warning: defaultThermalWarning, ceiling: defaultThermalCeiling}
	}
	{
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		obs = &thermalObserver{
			inner:   obs,
			warning: tb.warning,
			ceiling: tb.ceiling,
			cancel:  cancel,
		}
	}

	if obs != nil {
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			dg.observer = obs
		}
	}

	if cfg.hub != nil {
		if dg, ok := runner.Graph.(*DefaultGraph); ok {
			dg.hub = cfg.hub
		}
	}

	walker := cfg.walker
	if walker == nil {
		walker = circuit.NewProcessWalker("run")
	}

	if cfg.offsetPreamble != "" {
		applyOffsetPreamble(walker, cfg.offsetPreamble)
	}

	startNode := string(def.Start)

	if cfg.checkpointer != nil && cfg.resumeID != "" {
		startNode, err = resumeFromCheckpoint(cfg, walker, startNode)
		if err != nil {
			return err
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
		walker = &checkpointingWalker{inner: walker, cp: cfg.checkpointer, observer: obs}
	}

	if cfg.nodeCache != nil {
		ttls := buildCacheTTLs(def)
		if len(ttls) > 0 {
			walker = &cachingWalker{
				inner:       walker,
				cache:       cfg.nodeCache,
				cacheTTL:    ttls,
				circuitHash: fmt.Sprintf("%x", sha256.Sum256(data)),
				log:         cfg.logger,
			}
		}
	}

	err = runner.Walk(ctx, walker, startNode)

	if errors.Is(err, ErrWalkInterrupted) {
		return nil
	}

	if err == nil && cfg.checkpointer != nil {
		cpID := walker.State().ID
		if rmErr := cfg.checkpointer.Remove(cpID); rmErr != nil {
			slog.WarnContext(ctx, circuit.LogCheckpointRemoveFailed, slog.Any(circuit.LogKeyWalkerID, cpID), slog.Any(circuit.LogKeyError, rmErr.Error()))
		}
	}
	return err
}

// Resume loads a checkpoint and continues a previously interrupted walk.
// humanInput is injected into the walker context as "resume_input" before
// the walk resumes from the checkpointed node.
func Resume(ctx context.Context, circuitPath string, cp circuit.Checkpointer,
	walkerID string, humanInput any, opts ...RunOption) error {
	opts = append(opts, WithCheckpointer(cp), WithResumeInput(walkerID, humanInput))
	return Run(ctx, circuitPath, nil, opts...)
}

func resumeFromCheckpoint(cfg *runConfig, walker circuit.Walker, startNode string) (string, error) {
	loaded, loadErr := cfg.checkpointer.Load(cfg.resumeID)
	if loadErr != nil {
		return "", fmt.Errorf("load checkpoint %s: %w", cfg.resumeID, loadErr)
	}
	if loaded == nil {
		return startNode, nil
	}
	*walker.State() = *loaded
	startNode = loaded.CurrentNode
	if cfg.observer != nil {
		emitEvent(cfg.observer, &circuit.WalkEvent{Type: circuit.EventWalkResumed, Node: startNode, Walker: walker.Identity().Name})
	}
	return startNode, nil
}

// Validate loads and validates a circuit YAML without executing it.
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

	reg := &GraphRegistries{
		Nodes:       cfg.nodes,
		Edges:       cfg.edges,
		Extractors:  cfg.extractors,
		Instruments: cfg.instruments,
		Hooks:       cfg.hooks,
		Manifests:   cfg.manifests,
		Components:  cfg.components,
	}

	hasRegistries := reg.Nodes != nil || reg.Edges != nil || reg.Extractors != nil || reg.Instruments != nil || reg.Hooks != nil || reg.Manifests != nil
	if !hasRegistries {
		return nil
	}

	if _, err := BuildGraph(def, reg); err != nil {
		return fmt.Errorf("build graph (dry run): %w", err)
	}
	return validateNodeHooks(def, reg)
}

func validateNodeHooks(def *circuit.CircuitDef, reg *GraphRegistries) error {
	if reg.Hooks == nil {
		return nil
	}
	for i := range def.Nodes {
		for _, hookName := range def.Nodes[i].After {
			if _, hErr := reg.Hooks.Get(hookName); hErr != nil {
				return fmt.Errorf("node %q: hook %q: %w", def.Nodes[i].Name, hookName, hErr)
			}
		}
	}
	return nil
}

// WithOutputCapture attaches a WalkObserver as a capture observer.
// If another observer is already set, both are composed via MultiObserver.
func WithOutputCapture(capture circuit.WalkObserver) RunOption {
	return func(c *runConfig) {
		if c.observer == nil {
			c.observer = capture
		} else {
			c.observer = circuit.MultiObserver{c.observer, capture}
		}
	}
}

// applyOffsetPreamble is a no-op — PromptPreamble was removed.
func applyOffsetPreamble(_ circuit.Walker, _ string) {}

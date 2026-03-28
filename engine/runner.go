package engine

// Category: Execution

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/origami/circuit"
)

// Interrupt signals that a walk should pause at the current node for
// human-in-the-loop review. When a walker's Handle returns an Interrupt,
// the runner checkpoints state and stops without error.
type Interrupt struct {
	Reason string
	Data   map[string]any
}

func (i Interrupt) Error() string {
	if i.Reason != "" {
		return "interrupt: " + i.Reason
	}
	return "interrupt"
}

// IsInterrupt checks whether an error is an Interrupt signal.
func IsInterrupt(err error) bool {
	var i Interrupt
	return errors.As(err, &i)
}

// AsInterrupt extracts the Interrupt from an error, if present.
func AsInterrupt(err error) (Interrupt, bool) {
	var i Interrupt
	ok := errors.As(err, &i)
	return i, ok
}

// Runner drives a circuit graph with automatic artifact schema validation,
// before-hooks (context injection), and after-hooks (side effects).
// Domain tools create a Runner from a circuit.CircuitDef and their registries,
// then call Walk with a domain Walker.
type Runner struct {
	Circuit    *circuit.CircuitDef
	Graph      Graph
	Schemas    map[string]*circuit.ArtifactSchema // node name -> schema (from circuit.CircuitDef)
	NodeBefore map[string][]string        // node name -> before-hook names (from circuit.NodeDef.Before)
	NodeHooks  map[string][]string        // node name -> after-hook names (from circuit.NodeDef.After)
	Hooks      HookRegistry               // resolved hooks
	Logger     *slog.Logger
}

// NewRunner constructs a Runner from a circuit definition and registries.
// Backward-compatible: accepts (NodeRegistry, EdgeFactory, ...ExtractorRegistry).
func NewRunner(def *circuit.CircuitDef, nodes NodeRegistry, edges EdgeFactory, extractors ...ExtractorRegistry) (*Runner, error) {
	var extReg ExtractorRegistry
	if len(extractors) > 0 {
		extReg = extractors[0]
	}
	return NewRunnerWith(def, &GraphRegistries{
		Nodes:      nodes,
		Edges:      edges,
		Extractors: extReg,
	})
}

// NewRunnerWith constructs a Runner using the full registries bundle.
func NewRunnerWith(def *circuit.CircuitDef, reg *GraphRegistries) (*Runner, error) {
	graph, err := BuildGraph(def, reg)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	schemas := make(map[string]*circuit.ArtifactSchema, len(def.Nodes))
	nodeBefore := make(map[string][]string, len(def.Nodes))
	nodeHooks := make(map[string][]string, len(def.Nodes))
	nodeConfigs := make(map[string]*circuit.NodeConfig, len(def.Nodes))
	needsFileWrite := false
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		if nd.Schema != nil {
			schemas[nd.Name] = nd.Schema
		}
		if len(nd.Before) > 0 {
			nodeBefore[nd.Name] = nd.Before
		}
		if len(nd.After) > 0 {
			nodeHooks[nd.Name] = nd.After
			for _, h := range nd.After {
				if h == BuiltinHookFileWrite {
					needsFileWrite = true
				}
			}
		}
		cfg := nd.EffectiveConfig()
		if cfg.OutputPath != "" || cfg.SQLiteQuery != "" {
			nodeConfigs[nd.Name] = cfg
		}
	}

	hooks := reg.Hooks
	if needsFileWrite {
		if hooks == nil {
			hooks = make(HookRegistry)
		}
		if _, err := hooks.Get(BuiltinHookFileWrite); err != nil {
			hooks.Register(&FileWriteHook{NodeConfigs: nodeConfigs})
		}
	}

	return &Runner{
		Circuit:    def,
		Graph:      graph,
		Schemas:    schemas,
		NodeBefore: nodeBefore,
		NodeHooks:  nodeHooks,
		Hooks:      hooks,
	}, nil
}

// Walk traverses the graph with the given walker, validating artifacts
// against declared schemas and firing after-hooks.
// If walker is nil, a ProcessWalker is used (delegates to node.Process()).
// Chain: hookingWalker -> validatingWalker -> inner walker.
func (r *Runner) Walk(ctx context.Context, walker circuit.Walker, startNode string) error {
	if walker == nil {
		walker = circuit.NewProcessWalker("default")
	}
	vw := &validatingWalker{
		inner:   walker,
		schemas: r.Schemas,
		log:     r.Logger,
	}
	var w circuit.Walker = vw
	hasHooks := (len(r.NodeBefore) > 0 || len(r.NodeHooks) > 0) && r.Hooks != nil
	if hasHooks {
		w = &hookingWalker{
			inner:      vw,
			nodeBefore: r.NodeBefore,
			nodeHooks:  r.NodeHooks,
			hooks:      r.Hooks,
			log:        r.Logger,
		}
	}
	return r.Graph.Walk(ctx, w, startNode)
}

// validatingWalker wraps a domain Walker to add schema validation
// after each Handle call.
type validatingWalker struct {
	inner   circuit.Walker
	schemas map[string]*circuit.ArtifactSchema
	log     *slog.Logger
}

func (vw *validatingWalker) Identity() circuit.AgentIdentity     { return vw.inner.Identity() }
func (vw *validatingWalker) SetIdentity(id *circuit.AgentIdentity) { vw.inner.SetIdentity(id) }
func (vw *validatingWalker) State() *circuit.WalkerState          { return vw.inner.State() }

func (vw *validatingWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	artifact, err := vw.inner.Handle(ctx, node, nc)
	if err != nil {
		return nil, err
	}

	schema, hasSchema := vw.schemas[node.Name()]
	if !hasSchema || schema == nil {
		return artifact, nil
	}

	if err := ValidateArtifact(schema, artifact); err != nil {
		if vw.log != nil {
			vw.log.WarnContext(ctx, "artifact schema validation failed",
				"node", node.Name(),
				"error", err.Error(),
			)
		}
		return nil, fmt.Errorf("node %s: artifact schema violation: %w", node.Name(), err)
	}

	return artifact, nil
}

// hookingWalker wraps a Walker to invoke before-hooks (context injection)
// and after-hooks (side effects). Before-hooks run with nil artifact and
// can inject data into walker context. After-hooks run with the node's
// artifact. Hook errors are logged but do not stop the walk by default.
type hookingWalker struct {
	inner      circuit.Walker
	nodeBefore map[string][]string // node name -> before-hook names
	nodeHooks  map[string][]string // node name -> after-hook names
	hooks      HookRegistry
	log        *slog.Logger

	// onHookEvent is an optional callback fired after each hook execution.
	// Parameters: hook name, phase ("before"/"after"/"veto"), error (nil on success).
	// Used to bridge hook events to a SignalBus without importing dispatch/.
	onHookEvent func(name, phase string, err error)
}

func (hw *hookingWalker) Identity() circuit.AgentIdentity     { return hw.inner.Identity() }
func (hw *hookingWalker) SetIdentity(id *circuit.AgentIdentity) { hw.inner.SetIdentity(id) }
func (hw *hookingWalker) State() *circuit.WalkerState          { return hw.inner.State() }

func (hw *hookingWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	hookCtx := WithWalkerState(ctx, hw.State())
	for _, name := range hw.nodeBefore[node.Name()] {
		hook, hErr := hw.hooks.Get(name)
		if hErr != nil {
			if hw.log != nil {
				hw.log.WarnContext(ctx, "before-hook not found", "hook", name, "node", node.Name())
			}
			continue
		}
		if hErr = hook.Run(hookCtx, node.Name(), nil); hErr != nil {
			if hw.log != nil {
				hw.log.WarnContext(ctx, "before-hook error", "hook", name, "node", node.Name(), "error", hErr.Error())
			}
		}
		if hw.onHookEvent != nil {
			hw.onHookEvent(name, "before", hErr)
		}
	}

	artifact, err := hw.inner.Handle(ctx, node, nc)
	if err != nil {
		return nil, err
	}

	for _, name := range hw.nodeHooks[node.Name()] {
		hook, hErr := hw.hooks.Get(name)
		if hErr != nil {
			if hw.log != nil {
				hw.log.WarnContext(ctx, "hook not found", "hook", name, "node", node.Name())
			}
			continue
		}
		hErr = hook.Run(hookCtx, node.Name(), artifact)
		if hErr != nil && errors.Is(hErr, circuit.ErrFindingVeto) {
			artifact = &VetoArtifact{Inner: artifact}
			if hw.onHookEvent != nil {
				hw.onHookEvent(name, "veto", hErr)
			}
			continue
		}
		if hErr != nil && hw.log != nil {
			hw.log.WarnContext(ctx, "hook error", "hook", name, "node", node.Name(), "error", hErr.Error())
		}
		if hw.onHookEvent != nil {
			hw.onHookEvent(name, "after", hErr)
		}
	}

	return artifact, nil
}

// WrapWithCheckpointer wraps a Walker so that state is saved after each
// successful node and on Interrupt. Use this when calling Runner.Walk()
// directly (outside of framework.Run) and you need checkpoint support.
func WrapWithCheckpointer(w circuit.Walker, cp circuit.Checkpointer) circuit.Walker {
	return &checkpointingWalker{inner: w, cp: cp}
}

// checkpointingWalker wraps a Walker to save state after each successful
// node Handle. This is the outermost wrapper in the walker chain.
type checkpointingWalker struct {
	inner circuit.Walker
	cp    circuit.Checkpointer
}

func (cw *checkpointingWalker) Identity() circuit.AgentIdentity     { return cw.inner.Identity() }
func (cw *checkpointingWalker) SetIdentity(id *circuit.AgentIdentity) { cw.inner.SetIdentity(id) }
func (cw *checkpointingWalker) State() *circuit.WalkerState          { return cw.inner.State() }

func (cw *checkpointingWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	artifact, err := cw.inner.Handle(ctx, node, nc)
	if err != nil {
		if IsInterrupt(err) {
			_ = cw.cp.Save(cw.inner.State())
		}
		return nil, err
	}
	if cpErr := cw.cp.Save(cw.inner.State()); cpErr != nil {
		return nil, fmt.Errorf("checkpoint after node %s: %w", node.Name(), cpErr)
	}
	return artifact, nil
}

package engine

// Category: Execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// EvalAction determines the next step after an Operator evaluation.
type EvalAction string

const (
	ActionContinue EvalAction = "continue"
	ActionEscalate EvalAction = "escalate"
	ActionDone     EvalAction = "done"
)

// Goal describes the desired end-state for an Operator reconciliation loop.
type Goal struct {
	Description string         `json:"description" yaml:"description"`
	Constraints map[string]any `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// SystemState is the current observed state provided by Operator.Observe.
type SystemState struct {
	Artifacts map[string]circuit.Artifact `json:"-"`
	Iteration int                         `json:"iteration"`
	Elapsed   time.Duration               `json:"elapsed"`
}

// Evaluation is the result of Operator.Evaluate.
type Evaluation struct {
	Met      bool       `json:"met"`
	Progress float64    `json:"progress"`
	Action   EvalAction `json:"action"`
	Reason   string     `json:"reason,omitempty"`
}

// WalkResult captures the outcome of a single circuit walk within the
// reconciliation loop.
type WalkResult struct {
	Artifacts map[string]circuit.Artifact `json:"-"`
	Elapsed   time.Duration               `json:"elapsed"`
	Error     error                       `json:"error,omitempty"`
}

// Operator implements the Kubernetes-style reconciliation loop for agentic circuits.
type Operator interface {
	Observe(ctx context.Context) (SystemState, error)
	Reconcile(ctx context.Context, goal Goal, state SystemState) (*circuit.CircuitDef, error)
	Evaluate(ctx context.Context, goal Goal, result WalkResult) (Evaluation, error)
}

// OperatorObserver receives lifecycle events from RunOperator.
type OperatorObserver interface {
	OnObserve(SystemState)
	OnEvaluate(Evaluation)
	OnReconcile(*circuit.CircuitDef)
	OnWalkComplete(WalkResult)
}

// OperatorOption configures a RunOperator invocation.
type OperatorOption func(*operatorConfig)

type operatorConfig struct {
	maxIterations int
	observer      OperatorObserver
	walkObserver  circuit.WalkObserver
}

// WithMaxIterations sets a defense-in-depth cap on reconciliation iterations.
func WithMaxIterations(n int) OperatorOption {
	return func(c *operatorConfig) { c.maxIterations = n }
}

// WithOperatorObserver attaches a lifecycle observer to the reconciliation loop.
func WithOperatorObserver(obs OperatorObserver) OperatorOption {
	return func(c *operatorConfig) { c.observer = obs }
}

// WithWalkObserver attaches a walk-level observer to each circuit walk
// within the reconciliation loop.
func WithWalkObserver(obs circuit.WalkObserver) OperatorOption {
	return func(c *operatorConfig) { c.walkObserver = obs }
}

// ContainerStatus tracks the lifecycle of a CircuitContainer.
type ContainerStatus string

const (
	StatusPending   ContainerStatus = "pending"
	StatusRunning   ContainerStatus = "running"
	StatusSucceeded ContainerStatus = "succeeded"
	StatusFailed    ContainerStatus = "failed"
	StatusAborted   ContainerStatus = "aborted"
)

// CircuitContainer manages a single circuit instance's lifecycle.
type CircuitContainer interface {
	ID() string
	Def() *circuit.CircuitDef
	Status() ContainerStatus
	Walk(ctx context.Context, reg *GraphRegistries) (*WalkResult, error)
	Abort(reason string) error
	Artifacts() map[string]circuit.Artifact
}

// InMemoryContainer executes a circuit walk in-process.
type InMemoryContainer struct {
	id        string
	def       *circuit.CircuitDef
	mu        sync.Mutex
	status    ContainerStatus
	artifacts map[string]circuit.Artifact
	cancel    context.CancelFunc
	walkObs   circuit.WalkObserver
}

// NewInMemoryContainer creates a container in StatusPending.
func NewInMemoryContainer(id string, def *circuit.CircuitDef, walkObs circuit.WalkObserver) *InMemoryContainer {
	return &InMemoryContainer{
		id:      id,
		def:     def,
		status:  StatusPending,
		walkObs: walkObs,
	}
}

func (c *InMemoryContainer) ID() string               { return c.id }
func (c *InMemoryContainer) Def() *circuit.CircuitDef { return c.def }

func (c *InMemoryContainer) Status() ContainerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *InMemoryContainer) Artifacts() map[string]circuit.Artifact {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.artifacts
}

func (c *InMemoryContainer) Walk(ctx context.Context, reg *GraphRegistries) (*WalkResult, error) {
	c.mu.Lock()
	if c.status == StatusAborted {
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: %s: already aborted", ErrContainer, c.id)
	}
	c.status = StatusRunning
	walkCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()

	defer cancel()

	walkStart := time.Now()
	graph, err := BuildGraph(c.def, reg)
	if err != nil {
		c.setFailed()
		return nil, fmt.Errorf("container %s: build graph: %w", c.id, err)
	}

	if c.walkObs != nil {
		if dg, ok := graph.(*DefaultGraph); ok {
			dg.SetObserver(c.walkObs)
		}
	}

	walker := circuit.NewProcessWalker(c.id)
	walkErr := graph.Walk(walkCtx, walker, string(c.def.Start))

	result := &WalkResult{
		Artifacts: walker.State().Outputs,
		Elapsed:   time.Since(walkStart),
		Error:     walkErr,
	}

	c.mu.Lock()
	c.artifacts = result.Artifacts
	if walkErr != nil {
		c.status = StatusFailed
	} else {
		c.status = StatusSucceeded
	}
	c.mu.Unlock()

	return result, walkErr
}

func (c *InMemoryContainer) Abort(reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status != StatusPending && c.status != StatusRunning {
		return fmt.Errorf("%w: %s: cannot abort in status %s", ErrContainer, c.id, c.status)
	}
	c.status = StatusAborted
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *InMemoryContainer) setFailed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = StatusFailed
}

// RunOperator executes the Operator reconciliation loop.
func RunOperator(ctx context.Context, op Operator, goal Goal, reg *GraphRegistries, opts ...OperatorOption) error {
	cfg := &operatorConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if goal.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, goal.Timeout)
		defer cancel()
	}

	for iteration := 1; ; iteration++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		if cfg.maxIterations > 0 && iteration > cfg.maxIterations {
			return fmt.Errorf("%w: %d iterations", circuit.ErrMaxIterations, cfg.maxIterations)
		}

		state, err := op.Observe(ctx)
		if err != nil {
			return fmt.Errorf("observe (iteration %d): %w", iteration, err)
		}
		state.Iteration = iteration
		if cfg.observer != nil {
			cfg.observer.OnObserve(state)
		}

		initResult := WalkResult{Artifacts: state.Artifacts, Elapsed: state.Elapsed}
		eval, err := op.Evaluate(ctx, goal, initResult)
		if err != nil {
			return fmt.Errorf("evaluate (iteration %d): %w", iteration, err)
		}
		if cfg.observer != nil {
			cfg.observer.OnEvaluate(eval)
		}

		if eval.Met || eval.Action == ActionDone {
			return nil
		}
		if eval.Action == ActionEscalate {
			return fmt.Errorf("%w: %s", circuit.ErrEscalate, eval.Reason)
		}

		def, err := op.Reconcile(ctx, goal, state)
		if err != nil {
			return fmt.Errorf("reconcile (iteration %d): %w", iteration, err)
		}
		if cfg.observer != nil {
			cfg.observer.OnReconcile(def)
		}

		container := NewInMemoryContainer(
			fmt.Sprintf("operator-iter-%d", iteration),
			def,
			cfg.walkObserver,
		)

		result, walkErr := container.Walk(ctx, reg)
		if cfg.observer != nil && result != nil {
			cfg.observer.OnWalkComplete(*result)
		}

		if walkErr != nil {
			return fmt.Errorf("walk (iteration %d): %w", iteration, walkErr)
		}
	}
}

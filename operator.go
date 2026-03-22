package framework

// Category: Execution — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type EvalAction = engine.EvalAction

const (
	ActionContinue = engine.ActionContinue
	ActionEscalate = engine.ActionEscalate
	ActionDone     = engine.ActionDone
)

type Goal = engine.Goal
type SystemState = engine.SystemState
type Evaluation = engine.Evaluation
type WalkResult = engine.WalkResult
type Operator = engine.Operator
type OperatorObserver = engine.OperatorObserver
type OperatorOption = engine.OperatorOption

type ContainerStatus = engine.ContainerStatus

const (
	StatusPending   = engine.StatusPending
	StatusRunning   = engine.StatusRunning
	StatusSucceeded = engine.StatusSucceeded
	StatusFailed    = engine.StatusFailed
	StatusAborted   = engine.StatusAborted
)

type CircuitContainer = engine.CircuitContainer
type InMemoryContainer = engine.InMemoryContainer

var (
	WithMaxIterations    = engine.WithMaxIterations
	WithOperatorObserver = engine.WithOperatorObserver
	WithWalkObserver     = engine.WithWalkObserver
	NewInMemoryContainer = engine.NewInMemoryContainer
	RunOperator          = engine.RunOperator
)

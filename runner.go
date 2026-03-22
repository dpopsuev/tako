package framework

// Category: Execution — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type Interrupt = engine.Interrupt
type Runner = engine.Runner

var (
	IsInterrupt          = engine.IsInterrupt
	AsInterrupt          = engine.AsInterrupt
	NewRunner            = engine.NewRunner
	NewRunnerWith        = engine.NewRunnerWith
	WrapWithCheckpointer = engine.WrapWithCheckpointer
)

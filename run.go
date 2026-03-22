package framework

// Category: Execution — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type RunOption = engine.RunOption

var (
	WithTransformers       = engine.WithTransformers
	WithHooks              = engine.WithHooks
	WithExtractors         = engine.WithExtractors
	WithNodes              = engine.WithNodes
	WithEdges              = engine.WithEdges
	WithComponents         = engine.WithComponents
	WithOverrides          = engine.WithOverrides
	WithWalker             = engine.WithWalker
	WithTeam               = engine.WithTeam
	WithRunObserver        = engine.WithRunObserver
	WithLogger             = engine.WithLogger
	WithMemory             = engine.WithMemory
	WithTaggedMemory       = engine.WithTaggedMemory
	WithNodeCache          = engine.WithNodeCache
	WithCheckpointer       = engine.WithCheckpointer
	WithResume             = engine.WithResume
	WithResumeInput        = engine.WithResumeInput
	WithOffsetCompensation = engine.WithOffsetCompensation

	Run      = engine.Run
	Validate = engine.Validate
)

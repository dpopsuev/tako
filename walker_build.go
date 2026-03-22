package framework

// Category: DSL & Build — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

// ValidateElement checks that name is a recognized element and returns it.
var ValidateElement = engine.ValidateElement

// BuildWalkersFromDef constructs Walker instances from YAML walker definitions.
var BuildWalkersFromDef = engine.BuildWalkersFromDef

package framework

// Category: Core Primitives — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type Graph = engine.Graph
type Zone = engine.Zone
type DefaultGraph = engine.DefaultGraph
type GraphOption = engine.GraphOption

var (
	WithDoneNode     = engine.WithDoneNode
	WithObserver     = engine.WithObserver
	WithNodeTimeouts = engine.WithNodeTimeouts
	NewGraph         = engine.NewGraph
)

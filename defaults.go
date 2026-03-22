package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization.
var DefaultWalker = engine.DefaultWalker

// DefaultWalkerWithElement returns a default Walker with a custom element.
var DefaultWalkerWithElement = engine.DefaultWalkerWithElement

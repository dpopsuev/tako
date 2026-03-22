package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type Transformer = engine.Transformer
type DeterministicTransformer = engine.DeterministicTransformer
type TypedTransformer = engine.TypedTransformer
type TransformerContext = engine.TransformerContext
type TransformerRegistry = engine.TransformerRegistry

var (
	IsDeterministic     = engine.IsDeterministic
	TransformerFunc     = engine.TransformerFunc
	IsTransformerNode   = engine.IsTransformerNode
	TransformerNodeName = engine.TransformerNodeName
)

const (
	BuiltinTransformerGoTemplate  = engine.BuiltinTransformerGoTemplate
	BuiltinTransformerPassthrough = engine.BuiltinTransformerPassthrough
)

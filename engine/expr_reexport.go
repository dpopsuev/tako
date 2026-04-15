package engine

// Re-exports from engine/expr sub-package for backward compatibility.

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/expr"
)

// Type aliases for expression types.
type (
	ExprContext       = expr.ExprContext
	ExprState         = expr.ExprState
	SignalExprHelpers = expr.SignalExprHelpers
)

// CompileExpressionEdge is re-exported from engine/expr.
func CompileExpressionEdge(def *circuit.EdgeDef, config ...map[string]any) (circuit.Edge, error) {
	return expr.CompileExpressionEdge(def, config...)
}

// RunExprProgramForTest is re-exported from engine/expr.
var RunExprProgramForTest = expr.RunExprProgramForTest

// BuildExprContextForTest is re-exported from engine/expr.
var BuildExprContextForTest = expr.BuildExprContextForTest

// ArtifactToMapForTest is re-exported from engine/expr.
var ArtifactToMapForTest = expr.ArtifactToMapForTest

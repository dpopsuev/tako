// Package expr provides expression edge compilation and evaluation for
// circuit when: conditions. Uses expr-lang for typed expression evaluation.
package expr

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/dpopsuev/tako/circuit"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// ErrEdge is returned for edge compilation/evaluation failures.
var ErrEdge = errors.New("edge")

// ExprContext is the evaluation context passed to when: expressions.
type ExprContext struct {
	Output  map[string]any    `expr:"output"`
	State   ExprState         `expr:"state"`
	Config  map[string]any    `expr:"config"`
	Signals SignalExprHelpers `expr:"signals"`
}

// ExprState exposes walker state to expressions.
type ExprState struct {
	Loops   map[string]int `expr:"loops"`
	Current string         `expr:"current"`
}

// expressionEdge evaluates a compiled expr-lang program against artifact + state.
type expressionEdge struct {
	def     circuit.EdgeDef
	program *vm.Program
	config  map[string]any
}

// CompileExpressionEdge compiles a When expression with a typed environment.
func CompileExpressionEdge(def *circuit.EdgeDef, config ...map[string]any) (circuit.Edge, error) {
	if def.When == "" {
		return nil, fmt.Errorf("%w: %s: When expression is empty", ErrEdge, def.ID)
	}

	program, err := expr.Compile(def.When,
		expr.Env(ExprContext{}),
		expr.AsBool(),
	)
	if err != nil {
		return nil, fmt.Errorf("edge %s: compile expression %q: %w", def.ID, def.When, err)
	}

	var cfg map[string]any
	if len(config) > 0 {
		cfg = config[0]
	}

	return &expressionEdge{def: *def, program: program, config: cfg}, nil
}

// Program returns the compiled program for test inspection.
func (e *expressionEdge) Program() *vm.Program { return e.program }

func (e *expressionEdge) ID() string            { return e.def.ID }
func (e *expressionEdge) From() string          { return string(e.def.From) }
func (e *expressionEdge) To() string            { return string(e.def.To) }
func (e *expressionEdge) IsShortcut() bool      { return e.def.Shortcut }
func (e *expressionEdge) IsLoop() bool          { return e.def.Loop }
func (e *expressionEdge) IsParallel() bool      { return e.def.Parallel }
func (e *expressionEdge) MergeStrategy() string { return e.def.Merge }
func (e *expressionEdge) Expression() string    { return e.def.When }

func (e *expressionEdge) Evaluate(artifact circuit.Artifact, state *circuit.WalkerState) *circuit.Transition {
	ctx := buildExprContext(artifact, state, e.config)

	result, err := expr.Run(e.program, ctx)
	if err != nil {
		return nil
	}

	matched, ok := result.(bool)
	if !ok || !matched {
		return nil
	}

	return &circuit.Transition{
		NextNode:    string(e.def.To),
		Explanation: fmt.Sprintf("when: %s", e.def.When),
	}
}

// RunExprProgramForTest is exported for root-package test backward compatibility.
func RunExprProgramForTest(program *vm.Program, ctx ExprContext) (any, error) {
	return runExprProgram(program, ctx)
}

// runExprProgram runs a compiled expression program against a context.
func runExprProgram(program *vm.Program, ctx ExprContext) (any, error) {
	return expr.Run(program, ctx)
}

// buildExprContext creates the evaluation context from artifact and walker state.
func buildExprContext(artifact circuit.Artifact, state *circuit.WalkerState, config map[string]any) ExprContext {
	output := artifactToMap(artifact)

	loops := make(map[string]int)
	current := ""
	var collector circuit.FindingCollector
	if state != nil {
		for k, v := range state.LoopCounts {
			loops[k] = v
		}
		current = state.CurrentNode
		collector, _ = state.Context[circuit.FindingCollectorKey].(circuit.FindingCollector)
	}

	if config == nil {
		config = make(map[string]any)
	}

	return ExprContext{
		Output:  output,
		State:   ExprState{Loops: loops, Current: current},
		Config:  config,
		Signals: SignalExprHelpers{Collector: collector},
	}
}

// SignalExprHelpers exposes finding queries to when: expressions.
type SignalExprHelpers struct {
	Collector circuit.FindingCollector
}

// HasFinding returns true if any finding is at or above the given severity.
func (h SignalExprHelpers) HasFinding(severity string) bool {
	if h.Collector == nil {
		return false
	}
	threshold := circuit.FindingSeverity(severity)
	for _, f := range h.Collector.Findings() {
		if circuit.SeverityAtOrAbove(f.Severity, threshold) {
			return true
		}
	}
	return false
}

// FindingCount returns the number of findings at or above the given severity.
func (h SignalExprHelpers) FindingCount(severity string) int {
	if h.Collector == nil {
		return 0
	}
	threshold := circuit.FindingSeverity(severity)
	count := 0
	for _, f := range h.Collector.Findings() {
		if circuit.SeverityAtOrAbove(f.Severity, threshold) {
			count++
		}
	}
	return count
}

// FindingDomain returns true if any finding matches the domain glob pattern.
func (h SignalExprHelpers) FindingDomain(domain string) bool {
	if h.Collector == nil {
		return false
	}
	for _, f := range h.Collector.Findings() {
		if matched, _ := path.Match(domain, f.Domain); matched {
			return true
		}
	}
	return false
}

// ArtifactToMapForTest is exported for root-package test backward compatibility.
func ArtifactToMapForTest(artifact circuit.Artifact) map[string]any {
	return artifactToMap(artifact)
}

// BuildExprContextForTest is exported for root-package test backward compatibility.
func BuildExprContextForTest(artifact circuit.Artifact, state *circuit.WalkerState, config map[string]any) ExprContext {
	return buildExprContext(artifact, state, config)
}

// artifactToMap converts an Artifact's Raw() value to a map[string]any.
func artifactToMap(artifact circuit.Artifact) map[string]any {
	if artifact == nil {
		return make(map[string]any)
	}

	raw := artifact.Raw()
	if raw == nil {
		return make(map[string]any)
	}

	if m, ok := raw.(map[string]any); ok {
		return m
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return map[string]any{"_raw": raw}
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"_raw": raw}
	}

	return m
}

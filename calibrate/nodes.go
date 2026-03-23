package calibrate

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/bugle/element"
)

// CaseRunner runs a single calibration case through the domain circuit.
// Consumers implement this interface to connect their domain circuit
// (e.g., Asterisk's RCA circuit) to the generic calibration circuit.
type CaseRunner interface {
	RunCase(ctx context.Context, caseID string, input any) (any, error)
}

// CaseScorer scores a single case result against ground truth.
// Returns metric ID → value pairs for the case.
type CaseScorer interface {
	ScoreCase(caseResult, groundTruth any) (map[string]float64, error)
}

// CaseRunnerFunc adapts a function to the CaseRunner interface.
type CaseRunnerFunc func(ctx context.Context, caseID string, input any) (any, error)

func (f CaseRunnerFunc) RunCase(ctx context.Context, caseID string, input any) (any, error) {
	return f(ctx, caseID, input)
}

// CaseScorerFunc adapts a function to the CaseScorer interface.
type CaseScorerFunc func(caseResult, groundTruth any) (map[string]float64, error)

func (f CaseScorerFunc) ScoreCase(caseResult, groundTruth any) (map[string]float64, error) {
	return f(caseResult, groundTruth)
}

// CalibrationInput is placed in the walker context at key "input" before
// walking the calibration circuit. It provides all the configuration
// needed by the 7 calibration nodes.
type CalibrationInput struct {
	Scenario    string
	Transformer string
	Runs        int
	Cases       []CaseInput
	GroundTruth map[string]any // caseID → ground truth
	ScoreCard   *ScoreCard
	CaseRunner  CaseRunner
	CaseScorer  CaseScorer
	Parallel    int
}

// CaseInput describes a single case to calibrate.
type CaseInput struct {
	ID    string
	Input any
}

// --- Artifact types ---

type calibrateArtifact struct {
	typ  string
	data any
}

func (a *calibrateArtifact) Type() string      { return a.typ }
func (a *calibrateArtifact) Confidence() float64 { return 1.0 }
func (a *calibrateArtifact) Raw() any           { return a.data }

// CaseResultEntry holds the result of running and scoring a single case.
type CaseResultEntry struct {
	CaseID      string
	Result      any
	Scores      map[string]float64
	Error       string
}

// --- Node implementations ---

// LoadScenarioNode reads the CalibrationInput from walker context and
// validates it. Outputs the input as an artifact for downstream nodes.
type LoadScenarioNode struct{}

func (n *LoadScenarioNode) Name() string                 { return "load_scenario" }
func (n *LoadScenarioNode) ElementAffinity() element.Element { return element.ElementEarth }

func (n *LoadScenarioNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	raw, ok := nc.WalkerState.Context["input"]
	if !ok {
		return nil, fmt.Errorf("load_scenario: no input in walker context")
	}
	ci, ok := raw.(*CalibrationInput)
	if !ok {
		return nil, fmt.Errorf("load_scenario: input type %T, want *CalibrationInput", raw)
	}
	if ci.ScoreCard == nil {
		return nil, fmt.Errorf("load_scenario: ScoreCard is nil")
	}
	if ci.CaseRunner == nil {
		return nil, fmt.Errorf("load_scenario: CaseRunner is nil")
	}
	if ci.CaseScorer == nil {
		return nil, fmt.Errorf("load_scenario: CaseScorer is nil")
	}
	if len(ci.Cases) == 0 {
		return nil, fmt.Errorf("load_scenario: no cases")
	}
	return &calibrateArtifact{typ: "scenario", data: ci}, nil
}

// FanOutCasesNode takes the CalibrationInput and stores the case list
// in walker state for iteration by downstream nodes.
type FanOutCasesNode struct{}

func (n *FanOutCasesNode) Name() string                 { return "fan_out" }
func (n *FanOutCasesNode) ElementAffinity() element.Element { return element.ElementWater }

func (n *FanOutCasesNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	ci := extractInput(nc)
	if ci == nil {
		return nil, fmt.Errorf("fan_out: no CalibrationInput in context")
	}
	nc.WalkerState.Context["case_count"] = len(ci.Cases)
	return &calibrateArtifact{typ: "case_list", data: ci.Cases}, nil
}

// WalkCaseNode runs each case through the domain CaseRunner.
// Supports parallel execution when CalibrationInput.Parallel > 1.
type WalkCaseNode struct{}

func (n *WalkCaseNode) Name() string                 { return "walk_case" }
func (n *WalkCaseNode) ElementAffinity() element.Element { return element.ElementFire }

func (n *WalkCaseNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	ci := extractInput(nc)
	if ci == nil {
		return nil, fmt.Errorf("walk_case: no CalibrationInput in context")
	}

	results := make([]CaseResultEntry, len(ci.Cases))
	parallel := ci.Parallel
	if parallel <= 1 {
		for i, c := range ci.Cases {
			results[i] = runOneCase(ctx, ci.CaseRunner, c)
		}
	} else {
		var wg sync.WaitGroup
		sem := make(chan struct{}, parallel)
		for i, c := range ci.Cases {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, cs CaseInput) {
				defer wg.Done()
				defer func() { <-sem }()
				results[idx] = runOneCase(ctx, ci.CaseRunner, cs)
			}(i, c)
		}
		wg.Wait()
	}

	return &calibrateArtifact{typ: "case_results", data: results}, nil
}

func runOneCase(ctx context.Context, runner CaseRunner, c CaseInput) CaseResultEntry {
	result, err := runner.RunCase(ctx, c.ID, c.Input)
	entry := CaseResultEntry{CaseID: c.ID, Result: result}
	if err != nil {
		entry.Error = err.Error()
	}
	return entry
}

// ScoreCaseNode scores each case result against ground truth using CaseScorer.
type ScoreCaseNode struct{}

func (n *ScoreCaseNode) Name() string                 { return "score_case" }
func (n *ScoreCaseNode) ElementAffinity() element.Element { return element.ElementEarth }

func (n *ScoreCaseNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	ci := extractInput(nc)
	if ci == nil {
		return nil, fmt.Errorf("score_case: no CalibrationInput in context")
	}

	prior := nc.PriorArtifact
	if prior == nil {
		return nil, fmt.Errorf("score_case: no prior artifact (case_results)")
	}
	results, ok := prior.Raw().([]CaseResultEntry)
	if !ok {
		return nil, fmt.Errorf("score_case: prior artifact type %T, want []CaseResultEntry", prior.Raw())
	}

	for i := range results {
		if results[i].Error != "" {
			continue
		}
		gt := ci.GroundTruth[results[i].CaseID]
		scores, err := ci.CaseScorer.ScoreCase(results[i].Result, gt)
		if err != nil {
			results[i].Error = fmt.Sprintf("scoring: %v", err)
			continue
		}
		results[i].Scores = scores
	}

	return &calibrateArtifact{typ: "scored_results", data: results}, nil
}

// FanInResultsNode collects scored cases and produces per-metric value
// averages across all cases.
type FanInResultsNode struct{}

func (n *FanInResultsNode) Name() string                 { return "fan_in" }
func (n *FanInResultsNode) ElementAffinity() element.Element { return element.ElementWater }

func (n *FanInResultsNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	prior := nc.PriorArtifact
	if prior == nil {
		return nil, fmt.Errorf("fan_in: no prior artifact")
	}
	results, ok := prior.Raw().([]CaseResultEntry)
	if !ok {
		return nil, fmt.Errorf("fan_in: prior artifact type %T, want []CaseResultEntry", prior.Raw())
	}

	sums := make(map[string]float64)
	counts := make(map[string]int)
	for _, r := range results {
		if r.Error != "" {
			continue
		}
		for id, val := range r.Scores {
			sums[id] += val
			counts[id]++
		}
	}

	averages := make(map[string]float64, len(sums))
	for id, s := range sums {
		if counts[id] > 0 {
			averages[id] = s / float64(counts[id])
		}
	}

	nc.WalkerState.Context["case_results"] = results
	return &calibrateArtifact{typ: "metric_averages", data: averages}, nil
}

// AggregateNode evaluates metrics via the ScoreCard and computes the
// aggregate metric.
type AggregateNode struct{}

func (n *AggregateNode) Name() string                 { return "aggregate" }
func (n *AggregateNode) ElementAffinity() element.Element { return element.ElementEarth }

func (n *AggregateNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	ci := extractInput(nc)
	if ci == nil {
		return nil, fmt.Errorf("aggregate: no CalibrationInput in context")
	}
	prior := nc.PriorArtifact
	if prior == nil {
		return nil, fmt.Errorf("aggregate: no prior artifact")
	}
	averages, ok := prior.Raw().(map[string]float64)
	if !ok {
		return nil, fmt.Errorf("aggregate: prior artifact type %T, want map[string]float64", prior.Raw())
	}

	ms := ci.ScoreCard.Evaluate(averages, nil)
	if ci.ScoreCard.Aggregate != nil {
		agg, err := ci.ScoreCard.ComputeAggregate(ms)
		if err != nil {
			return nil, fmt.Errorf("aggregate: %w", err)
		}
		ms.Metrics = append(ms.Metrics, agg)
	}

	nc.WalkerState.Context["metric_set"] = ms
	return &calibrateArtifact{typ: "metric_set", data: ms}, nil
}

// ReportNode produces the final CalibrationReport from the aggregated metrics.
type ReportNode struct{}

func (n *ReportNode) Name() string                 { return "report" }
func (n *ReportNode) ElementAffinity() element.Element { return element.ElementAir }

func (n *ReportNode) Process(_ context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	ci := extractInput(nc)
	if ci == nil {
		return nil, fmt.Errorf("report: no CalibrationInput in context")
	}
	prior := nc.PriorArtifact
	if prior == nil {
		return nil, fmt.Errorf("report: no prior artifact")
	}
	ms, ok := prior.Raw().(MetricSet)
	if !ok {
		return nil, fmt.Errorf("report: prior artifact type %T, want MetricSet", prior.Raw())
	}

	report := &CalibrationReport{
		Scenario: ci.Scenario,
		Transformer: ci.Transformer,
		Runs:     ci.Runs,
		Metrics:  ms,
	}

	return &calibrateArtifact{typ: "calibration_report", data: report}, nil
}

// CalibrationNodeRegistry returns a NodeRegistry pre-loaded with all 7
// calibration circuit nodes, keyed by both family and name for flexible
// resolution. Consumers register this with BuildGraphWith.
func CalibrationNodeRegistry() engine.NodeRegistry {
	return engine.NodeRegistry{
		"calibrate.load":      func(_ circuit.NodeDef) circuit.Node { return &LoadScenarioNode{} },
		"calibrate.fan_out":   func(_ circuit.NodeDef) circuit.Node { return &FanOutCasesNode{} },
		"calibrate.walk_case": func(_ circuit.NodeDef) circuit.Node { return &WalkCaseNode{} },
		"calibrate.score_case": func(_ circuit.NodeDef) circuit.Node { return &ScoreCaseNode{} },
		"calibrate.fan_in":    func(_ circuit.NodeDef) circuit.Node { return &FanInResultsNode{} },
		"calibrate.aggregate": func(_ circuit.NodeDef) circuit.Node { return &AggregateNode{} },
		"calibrate.report":    func(_ circuit.NodeDef) circuit.Node { return &ReportNode{} },
		"load_scenario":       func(_ circuit.NodeDef) circuit.Node { return &LoadScenarioNode{} },
		"fan_out":             func(_ circuit.NodeDef) circuit.Node { return &FanOutCasesNode{} },
		"walk_case":           func(_ circuit.NodeDef) circuit.Node { return &WalkCaseNode{} },
		"score_case":          func(_ circuit.NodeDef) circuit.Node { return &ScoreCaseNode{} },
		"fan_in":              func(_ circuit.NodeDef) circuit.Node { return &FanInResultsNode{} },
		"aggregate":           func(_ circuit.NodeDef) circuit.Node { return &AggregateNode{} },
		"report":              func(_ circuit.NodeDef) circuit.Node { return &ReportNode{} },
	}
}

func extractInput(nc circuit.NodeContext) *CalibrationInput {
	raw, ok := nc.WalkerState.Context["input"]
	if !ok {
		return nil
	}
	ci, ok := raw.(*CalibrationInput)
	if !ok {
		return nil
	}
	return ci
}

// MarshalReport serializes a CalibrationReport to JSON.
func MarshalReport(report *CalibrationReport) ([]byte, error) {
	return json.Marshal(report)
}

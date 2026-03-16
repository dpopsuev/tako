package calibrate

import (
	"context"
	"fmt"
	"log/slog"

	framework "github.com/dpopsuev/origami"
)

// ScenarioLoader prepares domain-specific scenarios for BatchWalk.
// Each call to Load returns a fresh set of cases (e.g., with a new
// in-memory store), enabling independent multi-run calibration.
type ScenarioLoader interface {
	Load(ctx context.Context) ([]framework.BatchCase, error)
}

// CaseCollector extracts domain-specific results from BatchWalk output
// and produces metric values for ScoreCard evaluation.
// Implementations typically store domain state internally (e.g., per-case
// results) that callers retrieve after Run() for post-processing.
type CaseCollector interface {
	Collect(ctx context.Context, results []framework.BatchWalkResult) (
		values map[string]float64, details map[string]string, err error)
}

// ContractFieldsReceiver is optionally implemented by CaseCollectors that
// can consume contract-extracted fields. When the harness has a Contract,
// it calls SetContractFields before Collect so the collector can use
// pre-extracted values instead of hard-coding field paths.
type ContractFieldsReceiver interface {
	SetContractFields(fields []map[string]any)
}

// ReportRenderer produces human-readable output from a calibration report.
type ReportRenderer interface {
	Render(report *CalibrationReport) (string, error)
}

// HarnessConfig configures a generic calibration run.
type HarnessConfig struct {
	Loader    ScenarioLoader
	Collector CaseCollector
	Renderer  ReportRenderer

	CircuitDef *framework.CircuitDef
	ScoreCard  *ScoreCard
	Shared     framework.GraphRegistries

	// Contract enables contract-driven field extraction. When set, the
	// harness extracts scorer-addressable values from BatchWalkResults
	// using the contract's output mappings and stores them in
	// ContractFields (one map per case, keyed by scorer name). Domain
	// CaseCollectors can read ContractFields instead of hard-coding
	// field extraction logic.
	Contract *CalibrationContract

	// ContractFields is populated by Run() when Contract is set. Each
	// entry maps scorer_name to the extracted value for one case,
	// ordered the same as the BatchWalkResults. Read-only for callers.
	ContractFields []map[string]any

	// Resolution optionally sets the calibration resolution level.
	// When set alongside Plan, the harness records resolution metadata
	// in the report and passes port stubs to the circuit.
	Resolution Resolution
	Plan       *ResolutionPlan

	// PortStubs provides canned data for port boundaries during
	// isolated calibration. Keyed by port name (e.g. "rca.in:code-context").
	// Adapters check this map at port boundaries.
	PortStubs PortStubs

	Scenario    string
	Transformer string
	Runs        int
	Parallel    int

	OnCaseComplete func(index int, result framework.BatchWalkResult)
}

// Run orchestrates a generic calibration: load → walk → collect → score → aggregate.
// It returns the generic CalibrationReport. Domain-specific state (e.g., per-case
// results) is stored inside the CaseCollector and can be retrieved by the caller.
func Run(ctx context.Context, cfg HarnessConfig) (*CalibrationReport, error) {
	if cfg.Loader == nil {
		return nil, fmt.Errorf("calibrate.Run: Loader is required")
	}
	if cfg.Collector == nil {
		return nil, fmt.Errorf("calibrate.Run: Collector is required")
	}
	if cfg.CircuitDef == nil {
		return nil, fmt.Errorf("calibrate.Run: CircuitDef is required")
	}
	if cfg.ScoreCard == nil {
		return nil, fmt.Errorf("calibrate.Run: ScoreCard is required")
	}
	if cfg.Runs < 1 {
		cfg.Runs = 1
	}

	// Auto-derive contract from CircuitDef when not explicitly provided.
	if cfg.Contract == nil && cfg.CircuitDef.Calibration != nil {
		cfg.Contract = ContractFromDef(cfg.CircuitDef.Calibration)
	}

	// Inject port stubs into circuit Vars so adapters can read them at runtime.
	if len(cfg.PortStubs) > 0 {
		if cfg.CircuitDef.Vars == nil {
			cfg.CircuitDef.Vars = make(map[string]any)
		}
		cfg.CircuitDef.Vars["_port_stubs"] = cfg.PortStubs
	}

	logger := slog.Default().With("component", "calibrate")
	var allRunMetrics []MetricSet

	for run := 0; run < cfg.Runs; run++ {
		logger.Info("starting run", "run", run+1, "total", cfg.Runs)

		cases, err := cfg.Loader.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("run %d: load: %w", run+1, err)
		}

		batchResults := framework.BatchWalk(ctx, framework.BatchWalkConfig{
			Def:            cfg.CircuitDef,
			Shared:         cfg.Shared,
			Cases:          cases,
			Parallel:       cfg.Parallel,
			OnCaseComplete: cfg.OnCaseComplete,
		})

		if cfg.Contract != nil {
			cfg.ContractFields = make([]map[string]any, len(batchResults))
			for i, br := range batchResults {
				cfg.ContractFields[i] = ExtractFields(cfg.Contract, br)
			}
			if rcv, ok := cfg.Collector.(ContractFieldsReceiver); ok {
				rcv.SetContractFields(cfg.ContractFields)
			}
		}

		values, details, err := cfg.Collector.Collect(ctx, batchResults)
		if err != nil {
			return nil, fmt.Errorf("run %d: collect: %w", run+1, err)
		}

		ms := cfg.ScoreCard.Evaluate(values, details)
		if cfg.ScoreCard.Aggregate != nil {
			agg, err := cfg.ScoreCard.ComputeAggregate(ms)
			if err == nil {
				ms.Metrics = append(ms.Metrics, agg)
			}
		}

		allRunMetrics = append(allRunMetrics, ms)
	}

	report := &CalibrationReport{
		Scenario:    cfg.Scenario,
		Transformer: cfg.Transformer,
		Resolution:  string(cfg.Resolution),
		Runs:        cfg.Runs,
	}
	if cfg.Plan != nil {
		report.Plan = cfg.Plan.Name
	}

	eval := func(m Metric) bool {
		if def := cfg.ScoreCard.FindDef(m.ID); def != nil {
			return def.Evaluate(m.Value)
		}
		return m.Value >= m.Threshold
	}

	if len(allRunMetrics) == 1 {
		report.Metrics = allRunMetrics[0]
	} else {
		report.RunMetrics = allRunMetrics
		report.Metrics = AggregateRunMetrics(allRunMetrics, eval)
	}

	return report, nil
}

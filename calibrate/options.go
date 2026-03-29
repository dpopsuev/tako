package calibrate

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// HarnessOption configures a calibration harness.
type HarnessOption func(*HarnessConfig)

// NewHarnessConfig creates a HarnessConfig with the given options applied.
func NewHarnessConfig(opts ...HarnessOption) *HarnessConfig {
	cfg := &HarnessConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithLoader sets the ScenarioLoader that prepares cases for BatchWalk.
func WithLoader(l ScenarioLoader) HarnessOption {
	return func(c *HarnessConfig) { c.Loader = l }
}

// WithCollector sets the CaseCollector that extracts metrics from results.
func WithCollector(col CaseCollector) HarnessOption {
	return func(c *HarnessConfig) { c.Collector = col }
}

// WithRenderer sets the ReportRenderer for human-readable output.
func WithRenderer(r ReportRenderer) HarnessOption {
	return func(c *HarnessConfig) { c.Renderer = r }
}

// WithCircuitDef sets the circuit definition to walk.
func WithCircuitDef(def *circuit.CircuitDef) HarnessOption {
	return func(c *HarnessConfig) { c.CircuitDef = def }
}

// WithScoreCard sets the ScoreCard used to evaluate metrics.
func WithScoreCard(sc *ScoreCard) HarnessOption {
	return func(c *HarnessConfig) { c.ScoreCard = sc }
}

// WithShared sets the shared GraphRegistries for the harness.
func WithShared(reg *engine.GraphRegistries) HarnessOption {
	return func(c *HarnessConfig) { c.Shared = reg }
}

// WithContract sets the CalibrationContract for field extraction.
func WithContract(ct *CalibrationContract) HarnessOption {
	return func(c *HarnessConfig) { c.Contract = ct }
}

// WithResolution sets the calibration resolution level and plan.
func WithResolution(res Resolution, plan *ResolutionPlan) HarnessOption {
	return func(c *HarnessConfig) { c.Resolution = res; c.Plan = plan }
}

// WithPortStubs provides canned data for port boundaries during isolated calibration.
func WithPortStubs(stubs PortStubs) HarnessOption {
	return func(c *HarnessConfig) { c.PortStubs = stubs }
}

// WithComponents sets pre-built components to merge into shared registries.
func WithComponents(comps ...*engine.Component) HarnessOption {
	return func(c *HarnessConfig) { c.Components = comps }
}

// WithWalkerContext sets context values injected into every walker.
func WithWalkerContext(ctx map[string]any) HarnessOption {
	return func(c *HarnessConfig) { c.WalkerContext = ctx }
}

// WithPromptRelayer sets the mediator prompt relayer for sub-circuit delegation.
func WithPromptRelayer(r engine.PromptRelayer) HarnessOption {
	return func(c *HarnessConfig) { c.PromptRelayer = r }
}

// WithMaxErrorRate sets the maximum allowed fraction of cases with circuit errors.
func WithMaxErrorRate(rate float64) HarnessOption {
	return func(c *HarnessConfig) { c.MaxErrorRate = rate }
}

// WithScenario sets the scenario name for the calibration report.
func WithScenario(name string) HarnessOption {
	return func(c *HarnessConfig) { c.Scenario = name }
}

// WithTransformerName sets the transformer name for the calibration report.
func WithTransformerName(name string) HarnessOption {
	return func(c *HarnessConfig) { c.Transformer = name }
}

// WithRuns sets the number of calibration runs.
func WithRuns(n int) HarnessOption {
	return func(c *HarnessConfig) { c.Runs = n }
}

// WithParallel sets the number of parallel cases per run.
func WithParallel(n int) HarnessOption {
	return func(c *HarnessConfig) { c.Parallel = n }
}

// WithOnCaseComplete sets a callback invoked after each case completes.
func WithOnCaseComplete(fn func(int, engine.BatchWalkResult)) HarnessOption {
	return func(c *HarnessConfig) { c.OnCaseComplete = fn }
}

// WithHarnessObserver attaches a WalkObserver for debug/trace events.
// Named WithHarnessObserver to avoid collision with WithObserver in circuit.go.
func WithHarnessObserver(obs circuit.WalkObserver) HarnessOption {
	return func(c *HarnessConfig) { c.Observer = obs }
}

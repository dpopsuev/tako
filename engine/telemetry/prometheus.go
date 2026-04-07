package telemetry

import (
	"sync"
	"time"

	fw "github.com/dpopsuev/origami/circuit"
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusCollector translates WalkEvents into Prometheus metrics.
// It also exposes RecordTokens and RecordDispatch for bridging with
// the dispatch layer without creating import cycles.
type PrometheusCollector struct {
	NodeDuration    *prometheus.HistogramVec
	EdgeTransitions *prometheus.CounterVec
	WalkActive      *prometheus.GaugeVec
	WalkCompleted   *prometheus.CounterVec
	LoopsTotal      *prometheus.CounterVec

	TokensTotal   *prometheus.CounterVec
	TokensCostUSD *prometheus.CounterVec

	EvidenceSNR     *prometheus.GaugeVec
	WalkerMismatch  *prometheus.GaugeVec
	ConvergenceType *prometheus.CounterVec

	CircuitBreakerState *prometheus.GaugeVec
	RateLimitWaits      *prometheus.CounterVec
	ThermalBudgetUsed   *prometheus.GaugeVec

	DispatchDuration *prometheus.HistogramVec
	DispatchErrors   *prometheus.CounterVec

	Registry *prometheus.Registry

	mu      sync.Mutex
	circuit string
}

// NewPrometheusCollector creates a collector and registers metrics on the given registry.
// Pass nil to use a new default registry.
func NewPrometheusCollector(reg *prometheus.Registry) *PrometheusCollector {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}

	c := &PrometheusCollector{
		Registry: reg,
		NodeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "origami_walk_node_duration_seconds",
			Help:    "Duration of node processing in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"circuit", "node"}),
		EdgeTransitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_walk_edge_transitions_total",
			Help: "Total edge transitions.",
		}, []string{"circuit", "from", "to"}),
		WalkActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "origami_walk_active",
			Help: "Number of active walks.",
		}, []string{"circuit"}),
		WalkCompleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_walk_completed_total",
			Help: "Total completed walks.",
		}, []string{"circuit", "status"}),
		LoopsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_walk_loops_total",
			Help: "Total loop iterations.",
		}, []string{"circuit", "node"}),

		TokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_tokens_total",
			Help: "Total LLM tokens consumed.",
		}, []string{"circuit", "step", "node", "direction"}),
		TokensCostUSD: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_tokens_cost_usd",
			Help: "Estimated LLM token cost in USD.",
		}, []string{"circuit", "step"}),

		EvidenceSNR: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "origami_evidence_snr",
			Help: "Evidence signal-to-noise ratio per node.",
		}, []string{"circuit", "node"}),
		WalkerMismatch: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "origami_walker_mismatch",
			Help: "Walker-node impedance mismatch score (0=perfect, 1=worst).",
		}, []string{"circuit", "node"}),
		ConvergenceType: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_convergence_trajectory_total",
			Help: "Convergence trajectory classifications observed.",
		}, []string{"circuit", "type"}),

		CircuitBreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "origami_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open).",
		}, []string{"circuit", "provider"}),
		RateLimitWaits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_rate_limit_waits_total",
			Help: "Total dispatch calls delayed by rate limiting.",
		}, []string{"circuit"}),
		ThermalBudgetUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "origami_thermal_budget_used",
			Help: "Cumulative walk latency in seconds (thermal budget).",
		}, []string{"circuit"}),

		DispatchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "origami_dispatch_duration_seconds",
			Help:    "Duration of dispatch calls in seconds.",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		}, []string{"provider", "step"}),
		DispatchErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "origami_dispatch_errors_total",
			Help: "Total dispatch errors by provider and step.",
		}, []string{"provider", "step"}),
	}

	reg.MustRegister(
		c.NodeDuration, c.EdgeTransitions, c.WalkActive, c.WalkCompleted, c.LoopsTotal,
		c.TokensTotal, c.TokensCostUSD,
		c.EvidenceSNR, c.WalkerMismatch, c.ConvergenceType,
		c.CircuitBreakerState, c.RateLimitWaits, c.ThermalBudgetUsed,
		c.DispatchDuration, c.DispatchErrors,
	)
	return c
}

// SetCircuit configures the circuit label for subsequent events.
func (c *PrometheusCollector) SetCircuit(name string) {
	c.mu.Lock()
	c.circuit = name
	c.mu.Unlock()
}

//nolint:gocyclo // event type switch for Prometheus metrics — one case per event
func (c *PrometheusCollector) OnEvent(e *fw.WalkEvent) {
	c.mu.Lock()
	circuit := c.circuit
	c.mu.Unlock()

	switch e.Type {
	case fw.EventNodeExit:
		c.NodeDuration.WithLabelValues(circuit, e.Node).Observe(e.Elapsed.Seconds())
		if snr, ok := e.Metadata["snr"].(float64); ok {
			c.EvidenceSNR.WithLabelValues(circuit, e.Node).Set(snr)
		}
	case fw.EventTransition:
		from := ""
		to := ""
		if e.Metadata != nil {
			if f, ok := e.Metadata["from"].(string); ok {
				from = f
			}
			if t, ok := e.Metadata["to"].(string); ok {
				to = t
			}
		}
		if from == "" {
			from = e.Node
		}
		c.EdgeTransitions.WithLabelValues(circuit, from, to).Inc()
	case fw.EventWalkComplete:
		c.WalkActive.WithLabelValues(circuit).Dec()
		c.WalkCompleted.WithLabelValues(circuit, "success").Inc()
	case fw.EventWalkError:
		c.WalkActive.WithLabelValues(circuit).Dec()
		c.WalkCompleted.WithLabelValues(circuit, "error").Inc()
	case fw.EventNodeEnter:
		if circuit != "" {
			c.WalkActive.WithLabelValues(circuit).Add(0)
		}
	case fw.EventCircuitOpen:
		provider := ""
		if e.Metadata != nil {
			if p, ok := e.Metadata["provider"].(string); ok {
				provider = p
			}
		}
		c.CircuitBreakerState.WithLabelValues(circuit, provider).Set(1)
	case fw.EventCircuitClose:
		provider := ""
		if e.Metadata != nil {
			if p, ok := e.Metadata["provider"].(string); ok {
				provider = p
			}
		}
		c.CircuitBreakerState.WithLabelValues(circuit, provider).Set(0)
	case fw.EventRateLimit:
		c.RateLimitWaits.WithLabelValues(circuit).Inc()
	case fw.EventThermalWarning:
		if cumulative, ok := e.Metadata["cumulative"].(float64); ok {
			c.ThermalBudgetUsed.WithLabelValues(circuit).Set(cumulative)
		}
	}
}

// StartWalk increments the active walk gauge.
func (c *PrometheusCollector) StartWalk(circuit string) {
	c.SetCircuit(circuit)
	c.WalkActive.WithLabelValues(circuit).Inc()
}

// RecordTokens increments token counters for a dispatch step.
// direction should be "prompt" or "artifact".
func (c *PrometheusCollector) RecordTokens(step, node string, promptTokens, artifactTokens int, costUSD float64) {
	c.mu.Lock()
	circuit := c.circuit
	c.mu.Unlock()

	c.TokensTotal.WithLabelValues(circuit, step, node, "prompt").Add(float64(promptTokens))
	c.TokensTotal.WithLabelValues(circuit, step, node, "artifact").Add(float64(artifactTokens))
	c.TokensCostUSD.WithLabelValues(circuit, step).Add(costUSD)
}

// RecordSNR records an evidence SNR value for a node.
func (c *PrometheusCollector) RecordSNR(node string, snr float64) {
	c.mu.Lock()
	circuit := c.circuit
	c.mu.Unlock()
	c.EvidenceSNR.WithLabelValues(circuit, node).Set(snr)
}

// RecordMismatch records a walker-node mismatch score.
func (c *PrometheusCollector) RecordMismatch(node string, mismatch float64) {
	c.mu.Lock()
	circuit := c.circuit
	c.mu.Unlock()
	c.WalkerMismatch.WithLabelValues(circuit, node).Set(mismatch)
}

// RecordTrajectory increments the counter for a convergence trajectory type.
func (c *PrometheusCollector) RecordTrajectory(trajectoryType string) {
	c.mu.Lock()
	circuit := c.circuit
	c.mu.Unlock()
	c.ConvergenceType.WithLabelValues(circuit, trajectoryType).Inc()
}

// RecordDispatch records a dispatch duration and optional error.
func (c *PrometheusCollector) RecordDispatch(provider, step string, duration time.Duration, err error) {
	c.DispatchDuration.WithLabelValues(provider, step).Observe(duration.Seconds())
	if err != nil {
		c.DispatchErrors.WithLabelValues(provider, step).Inc()
	}
}

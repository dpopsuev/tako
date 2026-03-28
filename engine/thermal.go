package engine

// Category: Execution

import (
	"context"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

type thermalConfig struct {
	warning time.Duration
	ceiling time.Duration
}

// WithThermalBudget adds a cumulative latency budget to the walk.
func WithThermalBudget(warning, ceiling time.Duration) RunOption {
	return func(c *runConfig) {
		c.thermalBudget = &thermalConfig{warning: warning, ceiling: ceiling}
	}
}

// thermalObserver wraps another observer and tracks cumulative node latency.
type thermalObserver struct {
	inner   circuit.WalkObserver
	warning time.Duration
	ceiling time.Duration
	cancel  context.CancelFunc

	mu      sync.Mutex
	total   time.Duration
	warned  bool
	aborted bool
}

func (t *thermalObserver) OnEvent(e *circuit.WalkEvent) {
	if t.inner != nil {
		t.inner.OnEvent(e)
	}

	if e.Type != circuit.EventNodeExit || e.Error != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.total += e.Elapsed

	if !t.warned && t.warning > 0 && t.total >= t.warning {
		t.warned = true
		emitEvent(t.inner, &circuit.WalkEvent{
			Type: circuit.EventThermalWarning,
			Metadata: map[string]any{
				"cumulative": t.total.Seconds(),
				"warning":    t.warning.Seconds(),
				"ceiling":    t.ceiling.Seconds(),
			},
		})
	}

	if !t.aborted && t.total >= t.ceiling {
		t.aborted = true
		t.cancel()
	}
}

// Total returns the cumulative latency tracked so far.
func (t *thermalObserver) Total() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

package trace

// Category: Execution — bounded ring buffer for station-level tracing.

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// FlightEvent is a timestamped record of a station crossing.
type FlightEvent struct {
	Time    time.Time
	Station string // e.g. "walker:enter", "edge:evaluate", "session:create"
	Dir     string // "in" or "out"
	Summary string // human-readable: "case=case-1 node=triage"
	Data    any    // raw payload for deep inspection
	Err     error
}

// FlightRecorder is a bounded ring buffer that captures station crossings
// for post-mortem debugging. Implements circuit.WalkObserver for zero-wiring
// integration with the engine. Thread-safe.
type FlightRecorder struct {
	mu     sync.Mutex
	events []FlightEvent
	cap    int
	pos    int // write position (ring buffer)
	full   bool
}

// NewFlightRecorder creates a recorder with the given capacity.
// When full, oldest events are overwritten.
func NewFlightRecorder(capacity int) *FlightRecorder {
	if capacity <= 0 {
		capacity = 1000
	}
	return &FlightRecorder{
		events: make([]FlightEvent, capacity),
		cap:    capacity,
	}
}

// Record appends a flight event. If the buffer is full, the oldest event
// is overwritten (ring buffer semantics).
func (r *FlightRecorder) Record(station, dir, summary string, data any, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[r.pos] = FlightEvent{
		Time:    time.Now(),
		Station: station,
		Dir:     dir,
		Summary: summary,
		Data:    data,
		Err:     err,
	}
	r.pos = (r.pos + 1) % r.cap
	if r.pos == 0 {
		r.full = true
	}
}

// OnEvent implements circuit.WalkObserver. Maps walk events to station
// tags so the FlightRecorder captures engine-level events automatically.
func (r *FlightRecorder) OnEvent(e *circuit.WalkEvent) {
	station, dir := mapWalkEvent(e)
	summary := formatEventSummary(e)
	r.Record(station, dir, summary, e.Artifact, e.Error)
}

// Events returns all recorded events in chronological order.
func (r *FlightRecorder) Events() []FlightEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]FlightEvent, r.pos)
		copy(out, r.events[:r.pos])
		return out
	}
	// Ring buffer wrapped — read from pos to end, then 0 to pos.
	out := make([]FlightEvent, r.cap)
	copy(out, r.events[r.pos:])
	copy(out[r.cap-r.pos:], r.events[:r.pos])
	return out
}

// Query returns events matching the station prefix.
func (r *FlightRecorder) Query(stationPrefix string) []FlightEvent {
	all := r.Events()
	var out []FlightEvent
	for _, e := range all {
		if strings.HasPrefix(e.Station, stationPrefix) {
			out = append(out, e)
		}
	}
	return out
}

// Dump prints the full timeline to the test log. Call on failure for
// instant post-mortem diagnosis.
func (r *FlightRecorder) Dump(tb testing.TB) { //nolint:thelper // Dump is not a test helper — it's a diagnostic printer
	events := r.Events()
	if len(events) == 0 {
		tb.Log("=== FLIGHT RECORDER (empty) ===")
		return
	}
	tb.Logf("=== FLIGHT RECORDER (%d events) ===", len(events))
	base := events[0].Time
	for _, e := range events {
		elapsed := e.Time.Sub(base)
		errStr := ""
		if e.Err != nil {
			errStr = " ERR " + e.Err.Error()
		}
		tb.Logf("%8.3fs [%-20s] %-3s %s%s",
			elapsed.Seconds(), e.Station, e.Dir, e.Summary, errStr)
	}
}

// Reset clears all recorded events.
func (r *FlightRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pos = 0
	r.full = false
}

const (
	dirIn  = "in"
	dirOut = "out"
)

// mapWalkEvent translates a circuit.WalkEvent into a station tag + direction.
func mapWalkEvent(e *circuit.WalkEvent) (station, dir string) {
	switch e.Type {
	case circuit.EventNodeEnter:
		return "walker:enter", dirIn
	case circuit.EventNodeExit:
		return "walker:exit", dirOut
	case circuit.EventEdgeEvaluate:
		return "edge:evaluate", dirIn
	case circuit.EventTransition:
		return "edge:select", dirOut
	case circuit.EventWalkComplete:
		return "circuit:done", dirOut
	case circuit.EventWalkError:
		return "circuit:error", dirOut
	case circuit.EventWalkInterrupted:
		return "circuit:interrupted", dirOut
	case circuit.EventDelegateStart:
		return "delegate:start", dirIn
	case circuit.EventDelegateEnd:
		return "delegate:end", dirOut
	default:
		return fmt.Sprintf("unknown:%s", e.Type), dirIn
	}
}

// formatEventSummary produces a human-readable one-line summary.
func formatEventSummary(e *circuit.WalkEvent) string {
	var parts []string
	if e.Node != "" {
		parts = append(parts, "node="+e.Node)
	}
	if e.Walker != "" {
		parts = append(parts, "walker="+e.Walker)
	}
	if e.Edge != "" {
		parts = append(parts, "edge="+e.Edge)
	}
	if e.Elapsed > 0 {
		parts = append(parts, fmt.Sprintf("elapsed=%dms", e.Elapsed.Milliseconds()))
	}
	return strings.Join(parts, " ")
}

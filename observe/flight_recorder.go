package observe

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/tako/service/andon"
)

type FlightEvent struct {
	Time    time.Time
	Station string
	Dir     string
	Summary string
	Err     error
}

type FlightRecorder struct {
	mu     sync.Mutex
	events []FlightEvent
	cap    int
	pos    int
	full   bool
}

func NewFlightRecorder(capacity int) *FlightRecorder {
	if capacity <= 0 {
		capacity = 1000
	}
	return &FlightRecorder{
		events: make([]FlightEvent, capacity),
		cap:    capacity,
	}
}

var _ andon.Observer = (*FlightRecorder)(nil)

func (r *FlightRecorder) Record(station, dir, summary string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[r.pos] = FlightEvent{
		Time:    time.Now(),
		Station: station,
		Dir:     dir,
		Summary: summary,
		Err:     err,
	}
	r.pos = (r.pos + 1) % r.cap
	if r.pos == 0 {
		r.full = true
	}
}

func (r *FlightRecorder) OnEvent(e *andon.Event) {
	station, dir := mapAndonEvent(e)
	summary := formatAndonSummary(e)
	r.Record(station, dir, summary, e.Error)
}

func (r *FlightRecorder) Events() []FlightEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]FlightEvent, r.pos)
		copy(out, r.events[:r.pos])
		return out
	}
	out := make([]FlightEvent, r.cap)
	copy(out, r.events[r.pos:])
	copy(out[r.cap-r.pos:], r.events[:r.pos])
	return out
}

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

func (r *FlightRecorder) Dump(tb testing.TB) { //nolint:thelper
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

func mapAndonEvent(e *andon.Event) (station, dir string) {
	switch e.Type {
	case andon.NodeEnter:
		return "station:enter", dirIn
	case andon.NodeExit:
		return "station:exit", dirOut
	case andon.EdgeEvaluate:
		return "contract:evaluate", dirIn
	case andon.Transition:
		return "contract:pass", dirOut
	case andon.WalkComplete:
		return "fab:complete", dirOut
	case andon.WalkError:
		return "fab:error", dirOut
	case andon.Interrupted:
		return "fab:interrupted", dirOut
	case andon.DelegateStart:
		return "delegate:start", dirIn
	case andon.DelegateEnd:
		return "delegate:end", dirOut
	default:
		return fmt.Sprintf("unknown:%s", e.Type), dirIn
	}
}

func formatAndonSummary(e *andon.Event) string {
	var parts []string
	if e.Node != "" {
		parts = append(parts, "node="+e.Node)
	}
	if e.Agent != "" {
		parts = append(parts, "agent="+e.Agent)
	}
	if e.Edge != "" {
		parts = append(parts, "edge="+e.Edge)
	}
	if e.Elapsed > 0 {
		parts = append(parts, fmt.Sprintf("elapsed=%dms", e.Elapsed.Milliseconds()))
	}
	return strings.Join(parts, " ")
}

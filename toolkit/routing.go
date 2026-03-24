package toolkit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// RoutingEntry records a single dispatch routing decision.
// Schematics can embed this and add domain-specific fields.
type RoutingEntry struct {
	CaseID     string    `json:"case_id"`
	Step       string    `json:"step"`
	Color      string    `json:"color"`
	Timestamp  time.Time `json:"timestamp"`
	DispatchID int64     `json:"dispatch_id,omitempty"`
}

// RoutingLog is an ordered sequence of routing decisions.
type RoutingLog []RoutingEntry

// ForCase filters entries by case ID.
func (l RoutingLog) ForCase(id string) RoutingLog {
	var out RoutingLog
	for _, e := range l {
		if e.CaseID == id {
			out = append(out, e)
		}
	}
	return out
}

// ForStep filters entries by step name.
func (l RoutingLog) ForStep(step string) RoutingLog {
	var out RoutingLog
	for _, e := range l {
		if e.Step == step {
			out = append(out, e)
		}
	}
	return out
}

// Len returns the number of entries.
func (l RoutingLog) Len() int { return len(l) }

// SaveRoutingLog writes a routing log to a JSON file.
func SaveRoutingLog(path string, log RoutingLog) error {
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal routing log: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write routing log to %s: %w", path, err)
	}
	return nil
}

// LoadRoutingLog reads a routing log from a JSON file.
func LoadRoutingLog(path string) (RoutingLog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routing log from %s: %w", path, err)
	}
	var log RoutingLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("unmarshal routing log: %w", err)
	}
	return log, nil
}

// RoutingDiff represents a mismatch between expected and actual routing.
type RoutingDiff struct {
	CaseID   string `json:"case_id"`
	Step     string `json:"step"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

// CompareRoutingLogs compares two routing logs and returns differences.
func CompareRoutingLogs(expected, actual RoutingLog) []RoutingDiff {
	type key struct{ CaseID, Step string }
	actualMap := make(map[key]string, len(actual))
	for _, e := range actual {
		actualMap[key{e.CaseID, e.Step}] = e.Color
	}
	expectedMap := make(map[key]string, len(expected))

	var diffs []RoutingDiff
	for _, e := range expected {
		k := key{e.CaseID, e.Step}
		expectedMap[k] = e.Color
		ac, ok := actualMap[k]
		if !ok {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: e.Color, Actual: "<missing>"})
		} else if ac != e.Color {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: e.Color, Actual: ac})
		}
	}
	for _, e := range actual {
		k := key{e.CaseID, e.Step}
		if _, ok := expectedMap[k]; !ok {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: "<missing>", Actual: e.Color})
		}
	}
	return diffs
}

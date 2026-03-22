package circuit

// Category: Processing & Support

import (
	"context"
	"time"
)

// FindingSeverity classifies the impact level of an enforcer finding.
type FindingSeverity string

const (
	FindingInfo    FindingSeverity = "info"
	FindingWarning FindingSeverity = "warning"
	FindingError   FindingSeverity = "error"
)

var severityOrder = map[FindingSeverity]int{
	FindingInfo:    0,
	FindingWarning: 1,
	FindingError:   2,
}

// SeverityAtOrAbove returns true when have is at or above the threshold severity.
func SeverityAtOrAbove(have, threshold FindingSeverity) bool {
	return severityOrder[have] >= severityOrder[threshold]
}

// Finding is a typed observation produced by an enforcer during circuit execution.
// All three enforcement patterns (Hook, Signal, Parallel Circuit) produce the same type.
type Finding struct {
	Severity  FindingSeverity `json:"severity"`
	Domain    string          `json:"domain"`
	Source    string          `json:"source"`
	NodeName  string          `json:"node_name"`
	Message   string          `json:"message"`
	Evidence  map[string]any  `json:"evidence,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// FindingCollector accumulates findings during a walk.
type FindingCollector interface {
	Report(ctx context.Context, f Finding) error
	Findings() []Finding
}

// FindingCollectorKey is the well-known key used to store a FindingCollector
// in WalkerState.Context, making it available to expression edges and hooks.
const FindingCollectorKey = "__finding_collector"

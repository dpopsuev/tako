// Package operator implements a reconciliation loop that watches a repository
// and runs circuit executions to converge toward a desired state.
// Docker/Podman first, K8s later.
package operator

import "time"

// DesiredState declares what the operator reconciles toward.
type DesiredState struct {
	Manifest        string `yaml:"manifest"`        // board manifest path (e.g. "tako-sdlc.yaml")
	RepoPath        string `yaml:"repo_path"`       // repository root to operate on
	Scan            string `yaml:"scan"`            // "clean" or max findings count as string
	Build           string `yaml:"build"`           // "passing"
	Test            string `yaml:"test"`            // "passing"
	Vulnerabilities int    `yaml:"vulnerabilities"` // max allowed
}

// CurrentState is a snapshot of the repository's actual state.
type CurrentState struct {
	HeadSHA         string    `json:"head_sha"`
	ScanFindings    int       `json:"scan_findings"`
	BuildPassing    bool      `json:"build_passing"`
	TestPassing     bool      `json:"test_passing"`
	Vulnerabilities int       `json:"vulnerabilities"`
	ObservedAt      time.Time `json:"observed_at"`
}

// DriftResult describes the gap between desired and current state.
type DriftResult struct {
	Drifted bool     `json:"drifted"`
	Reasons []string `json:"reasons"`
}

// RunResult captures the outcome of a single circuit run.
type RunResult struct {
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// Observer snapshots the current state of the repository.
type Observer interface {
	Observe() (*CurrentState, error)
}

// Actor executes a circuit run to fix drift.
type Actor interface {
	Act(drift DriftResult) (*RunResult, error)
}

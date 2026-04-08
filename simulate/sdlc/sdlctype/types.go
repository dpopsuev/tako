// Package sdlctype defines typed output structs for SDLC circuit nodes.
// Shared by instruments and the simulate/sdlc package. No import cycle.
package sdlctype

// ScanResult is produced by the scan node (Oculus + Ordo).
type ScanResult struct {
	Findings []Finding `json:"findings"`
	Clean    bool      `json:"clean"`
}

// Finding is a single issue found by the scan node.
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// FixResult is produced by the fix node (LLM code generation).
type FixResult struct {
	Fixed        []string `json:"fixed"`                   // file paths modified
	Applied      string   `json:"applied"`                 // fix description
	WorktreePath string   `json:"worktree_path,omitempty"` // worktree path (when isolation is active)
	Branch       string   `json:"branch,omitempty"`        // branch name (when isolation is active)
}

// BuildResult is produced by the build node (go build).
type BuildResult struct {
	Pass   bool   `json:"pass"`
	Output string `json:"output"`
}

// TestResult is produced by the test node (go test).
type TestResult struct {
	Pass   bool   `json:"pass"`
	Total  int    `json:"total"`
	Failed int    `json:"failed"`
	Output string `json:"output"`
}

// DeployResult is produced by the deploy-canary node.
type DeployResult struct {
	Version string `json:"version"` // git hash
	Canary  bool   `json:"canary"`
}

// ValidateResult is produced by the validate node (Prometheus query).
type ValidateResult struct {
	Healthy   bool    `json:"healthy"`
	ErrorRate float64 `json:"error_rate"`
	LatencyMs float64 `json:"latency_ms"`
}

// HardenResult is produced by the harden node (vulncheck + pkgquery).
type HardenResult struct {
	Vulnerabilities int      `json:"vulnerabilities"`
	PinnedDeps      []string `json:"pinned_deps"`
}

// ReleaseResult is produced by the release node (git tag + changelog).
type ReleaseResult struct {
	Tag       string `json:"tag"`
	Changelog string `json:"changelog"`
}

// TeardownResult is produced by the teardown (finally) node.
type TeardownResult struct {
	Cleaned []string `json:"cleaned"` // resources removed
}

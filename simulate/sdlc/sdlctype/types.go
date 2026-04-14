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

// SelfReviewResult is produced by the self-review node.
// It validates that code changes address every requirement in the Scribe artifact.
type SelfReviewResult struct {
	AllVerified bool    `json:"all_verified"` // true only when every requirement has evidence
	Stamps      []Stamp `json:"stamps"`       // per-requirement verification evidence
}

// Stamp records whether a single requirement was satisfied by the code changes.
// Field maps to a Scribe artifact field (title, goal, or section name).
// Evidence is a "file:line" reference where the requirement is addressed.
type Stamp struct {
	Field    string `json:"field"`    // requirement source: "title", "goal", or section name
	Status   string `json:"status"`   // "verified" or "unverified"
	Evidence string `json:"evidence"` // "path/to/file:line" or empty if unverified
}

// --- V2 sub-circuit result types ---

// PollScribeResult is produced by the poll-scribe node (planning circuit).
type PollScribeResult struct {
	HasTask bool   `json:"has_task"`
	TaskID  string `json:"task_id,omitempty"`
	Title   string `json:"title,omitempty"`
}

// ResolveContextResult is produced by the resolve-context node (planning circuit).
type ResolveContextResult struct {
	Spec         map[string]any `json:"spec"`
	Rules        []string       `json:"rules,omitempty"`
	Architecture map[string]any `json:"architecture,omitempty"`
}

// GateResult is produced by gate passthrough nodes (plan-review, diff-review).
type GateResult struct {
	Approved bool `json:"approved"`
}

// CreateWorktreeResult is produced by the create-worktree node (coding circuit).
type CreateWorktreeResult struct {
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
}

// WriteTestResult is produced by the write-test node (coding circuit, RED phase).
type WriteTestResult struct {
	TestFiles []string `json:"test_files"`
}

// WriteCodeResult is produced by the write-code node (coding circuit, GREEN phase).
type WriteCodeResult struct {
	FilesChanged []string `json:"files_changed"`
}

// RefactorResult is produced by the refactor node (coding circuit, BLUE phase).
type RefactorResult struct {
	FilesChanged []string `json:"files_changed,omitempty"`
}

// LintResult is produced by the lint node (verifying circuit).
type LintResult struct {
	Pass   bool   `json:"pass"`
	Output string `json:"output"`
}

// SecurityScanResult is produced by the security-scan node (verifying circuit).
type SecurityScanResult struct {
	Clean    bool      `json:"clean"`
	Findings []Finding `json:"findings,omitempty"`
}

// MonitorHealthResult is produced by the monitor-health node (operating circuit).
type MonitorHealthResult struct {
	Healthy bool           `json:"healthy"`
	Metrics map[string]any `json:"metrics,omitempty"`
}

// PromoteResult is produced by the promote node (operating circuit).
type PromoteResult struct {
	Promoted bool `json:"promoted"`
}

// RollbackResult is produced by the rollback node (operating circuit).
type RollbackResult struct {
	RolledBack bool `json:"rolled_back"`
}

// FileBugResult is produced by the file-bug node (operating circuit).
type FileBugResult struct {
	BugID string `json:"bug_id"`
}

// MarkDoneResult is produced by the mark-done node (operating circuit).
type MarkDoneResult struct {
	Updated bool `json:"updated"`
}

// TeardownResult is produced by the teardown (finally) node.
type TeardownResult struct {
	Cleaned []string `json:"cleaned"` // resources removed
}

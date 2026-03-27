package ingest

import "context"

// Verifier checks whether a candidate's ground truth references are valid.
// Implementations query external systems (Jira status, GitHub PR merged).
type Verifier interface {
	Name() string
	Verify(ctx context.Context, record Record) (VerifyResult, error)
}

// VerifyResult reports whether a record passed verification.
type VerifyResult struct {
	Verified bool   `json:"verified"`
	Reason   string `json:"reason"`
}

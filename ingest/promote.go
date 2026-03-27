package ingest

import "context"

// Promoter appends verified candidates to the ground truth scenario file.
type Promoter interface {
	Promote(ctx context.Context, candidates []Candidate, scenarioPath string) (promoted int, err error)
}

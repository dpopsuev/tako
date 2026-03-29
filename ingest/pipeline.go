package ingest

import (
	"context"
	"fmt"
	"log/slog"
)

// PipelineOpts carries resolved implementations for the pipeline.
// Consumers resolve modules from the DatasetManifest into concrete types.
type PipelineOpts struct {
	Source    Source
	Matcher   Matcher
	Dedup     DedupStore
	Writer    CandidateWriter
	Verifiers []Verifier
	Promoter  Promoter
}

// PipelineResult summarizes what the pipeline did.
type PipelineResult struct {
	Discovered int `json:"discovered"`
	Matched    int `json:"matched"`
	Duplicates int `json:"duplicates"`
	Verified   int `json:"verified"`
	Promoted   int `json:"promoted"`
}

// RunPipeline executes the full dataset pipeline:
// discover → match → dedup → verify → promote.
func RunPipeline(ctx context.Context, manifest *DatasetManifest, opts *PipelineOpts) (*PipelineResult, error) {
	if opts.Source == nil {
		return nil, ErrPipelineSourceIsRequired
	}
	if opts.Matcher == nil {
		return nil, ErrPipelineMatcherIsRequired
	}

	logger := slog.Default().With(slog.Any("component", "dataset-pipeline"), slog.Any("scenario", manifest.Metadata.Scenario))

	// Step 1: Ingest — discover → match → dedup → write candidates.
	cfg := Config{
		Extra: map[string]any{
			"scenario": manifest.Metadata.Scenario,
		},
	}

	dedup := opts.Dedup
	if dedup == nil {
		dedup = NewDedupIndex()
	}

	writer := opts.Writer
	if writer == nil {
		writer = &collectWriter{}
	}
	cw, isCollect := writer.(*collectWriter)

	summary, err := Run(ctx, cfg, opts.Source, opts.Matcher, dedup, writer)
	if err != nil {
		return nil, fmt.Errorf("pipeline ingest: %w", err)
	}

	result := &PipelineResult{
		Discovered: summary.Discovered,
		Matched:    summary.Matched,
		Duplicates: summary.Duplicates,
	}

	logger.InfoContext(ctx, "ingest complete", slog.Any("discovered", result.Discovered), slog.Any("matched", result.Matched), slog.Any("duplicates", result.Duplicates))

	// Get candidates for verification.
	var candidates []Candidate
	if isCollect {
		candidates = cw.candidates
	}
	if len(candidates) == 0 {
		return result, nil
	}

	// Step 2: Verify — check each candidate against external systems.
	var verified []Candidate
	for ci := range candidates {
		cand := &candidates[ci]
		allPassed := true
		for _, v := range opts.Verifiers {
			vr, verifyErr := v.Verify(ctx, cand.Record)
			if verifyErr != nil {
				logger.WarnContext(ctx, "verifier failed", slog.Any("verifier", v.Name()), slog.Any("candidate", cand.ID), slog.Any("error", verifyErr))
				allPassed = false
				break
			}
			if !vr.Verified {
				logger.InfoContext(ctx, "candidate not verified", slog.Any("verifier", v.Name()), slog.Any("candidate", cand.ID), slog.Any("reason", vr.Reason))
				allPassed = false
				break
			}
		}
		if allPassed {
			verified = append(verified, *cand)
		}
	}
	result.Verified = len(verified)

	logger.InfoContext(ctx, "verification complete", slog.Any("verified", result.Verified), slog.Any("total", len(candidates)))

	// Step 3: Promote — append verified candidates to scenario YAML.
	if opts.Promoter != nil && len(verified) > 0 && manifest.Output.Scenario != "" {
		promoted, promoteErr := opts.Promoter.Promote(ctx, verified, manifest.Output.Scenario)
		if promoteErr != nil {
			return result, fmt.Errorf("pipeline promote: %w", promoteErr)
		}
		result.Promoted = promoted
		logger.InfoContext(ctx, "promotion complete", slog.Any("promoted", result.Promoted))
	}

	return result, nil
}

// collectWriter is an in-memory CandidateWriter used when no external writer
// is provided. It collects candidates for the verify step.
type collectWriter struct {
	candidates []Candidate
}

func (w *collectWriter) Write(_ context.Context, candidates []Candidate) error {
	w.candidates = append(w.candidates, candidates...)
	return nil
}

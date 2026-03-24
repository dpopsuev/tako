// Package ingest provides a generic ETL pipeline for ingesting data from
// external sources into calibration datasets. Schematics implement the
// Source, Matcher, and CandidateWriter interfaces for their domain;
// the Run orchestrator chains them with deduplication.
package ingest

import (
	"context"
	"fmt"
	"time"
)

// Record is a generic ingestion record from any external source.
type Record struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`
	Fields   map[string]any `json:"fields"`
	DedupKey string         `json:"dedup_key"`
}

// Candidate is a matched record ready for human review.
type Candidate struct {
	ID        string         `json:"id"`
	Record    Record         `json:"record"`
	PatternID string         `json:"pattern_id,omitempty"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	DedupKey  string         `json:"dedup_key"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// Summary captures pipeline execution statistics.
type Summary struct {
	Discovered int `json:"discovered"`
	Matched    int `json:"matched"`
	Duplicates int `json:"duplicates"`
	Written    int `json:"written"`
}

// Source discovers and fetches raw records from an external system.
type Source interface {
	Discover(ctx context.Context, cfg Config) ([]Record, error)
}

// Matcher filters and enriches records by matching them against
// domain-specific patterns (symptoms, CVE signatures, etc.).
// Returns matched records with PatternID populated in DedupKey or Fields,
// and unmatched records separately.
type Matcher interface {
	Match(records []Record) (matched, unmatched []Record)
}

// DedupStore tracks which records have already been ingested.
type DedupStore interface {
	Contains(key string) bool
	Add(key string)
}

// CandidateWriter persists matched, deduplicated records for human review.
type CandidateWriter interface {
	Write(ctx context.Context, candidates []Candidate) error
}

// Run orchestrates the ETL pipeline: discover → match → dedup → write.
func Run(ctx context.Context, cfg Config, src Source, m Matcher, dedup DedupStore, w CandidateWriter) (*Summary, error) {
	records, err := src.Discover(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ingest discover: %w", err)
	}

	summary := &Summary{Discovered: len(records)}

	if len(records) == 0 {
		return summary, nil
	}

	matched, _ := m.Match(records)
	summary.Matched = len(matched)

	if len(matched) == 0 {
		return summary, nil
	}

	var candidates []Candidate
	now := time.Now()

	for i, rec := range matched {
		key := rec.DedupKey
		if dedup.Contains(key) {
			summary.Duplicates++
			continue
		}
		dedup.Add(key)

		patternID, _ := rec.Fields["pattern_id"].(string)

		candidates = append(candidates, Candidate{
			ID:        fmt.Sprintf("CAND-%d-%d", now.Unix(), i+1),
			Record:    rec,
			PatternID: patternID,
			Status:    "candidate",
			CreatedAt: now,
			DedupKey:  key,
		})
	}

	if len(candidates) == 0 {
		return summary, nil
	}

	if err := w.Write(ctx, candidates); err != nil {
		return summary, fmt.Errorf("ingest write: %w", err)
	}
	summary.Written = len(candidates)

	return summary, nil
}

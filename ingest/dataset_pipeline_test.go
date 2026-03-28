package ingest_test

import (
	"context"
	"os"
	"testing"

	"github.com/dpopsuev/origami/ingest"
)

func TestDatasetManifest_ParsesConsumerYAML(t *testing.T) {
	data, err := os.ReadFile("../testdata/dataset.yaml")
	if err != nil {
		home, _ := os.UserHomeDir()
		data, err = os.ReadFile(home + "/Workspace/asterisk/dataset.yaml")
		if err != nil {
			t.Skip("dataset.yaml not found")
		}
	}

	m, err := ingest.ParseDatasetManifest(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if m.Kind != "dataset" {
		t.Errorf("kind = %q, want dataset", m.Kind)
	}
	if m.Metadata.Scenario == "" {
		t.Error("metadata.scenario required")
	}
	if _, ok := m.Sources["reportportal"]; !ok {
		t.Error("sources must include reportportal")
	}
	if _, ok := m.Verification["jira"]; !ok {
		t.Error("verification must include jira")
	}
	if _, ok := m.Verification["github"]; !ok {
		t.Error("verification must include github")
	}
	if m.Output.Scenario == "" {
		t.Error("output.scenario required")
	}

	t.Logf("manifest: name=%s scenario=%s sources=%d verifiers=%d",
		m.Metadata.Name, m.Metadata.Scenario, len(m.Sources), len(m.Verification))
}

// --- Stubs for GREEN phase ---

type stubSource struct {
	records []ingest.Record
}

func (s *stubSource) Discover(_ context.Context, _ ingest.Config) ([]ingest.Record, error) {
	return s.records, nil
}

type stubMatcher struct{}

func (m *stubMatcher) Match(records []ingest.Record) (matched, unmatched []ingest.Record) {
	// Accept all records.
	return records, nil
}

type stubVerifier struct{ name string }

func (v *stubVerifier) Name() string { return v.name }
func (v *stubVerifier) Verify(_ context.Context, _ ingest.Record) (ingest.VerifyResult, error) {
	return ingest.VerifyResult{Verified: true, Reason: "stub"}, nil
}

type stubPromoter struct{ count int }

func (p *stubPromoter) Promote(_ context.Context, candidates []ingest.Candidate, _ string) (int, error) {
	p.count = len(candidates)
	return p.count, nil
}

func TestDatasetPipeline_RunSync(t *testing.T) {
	manifest := &ingest.DatasetManifest{
		Kind:    "dataset",
		Version: "v1",
		Metadata: ingest.ManifestMetadata{
			Name:     "test-pipeline",
			Scenario: "ptp",
		},
		Output: ingest.ManifestOutput{
			Scenario: "scenarios/ptp.yaml",
		},
	}

	src := &stubSource{
		records: []ingest.Record{
			{ID: "F1", Source: "rp", DedupKey: "rp-launch-100-item-1", Fields: map[string]any{
				"test_name":     "TestPTPClockSync",
				"error_message": "clock offset exceeded threshold",
				"launch_id":     100,
			}},
			{ID: "F2", Source: "rp", DedupKey: "rp-launch-100-item-2", Fields: map[string]any{
				"test_name":     "TestPTPEvents",
				"error_message": "HTTP events consumer timeout",
				"launch_id":     100,
			}},
		},
	}

	promoter := &stubPromoter{}

	result, err := ingest.RunPipeline(context.Background(), manifest, &ingest.PipelineOpts{
		Source:    src,
		Matcher:   &stubMatcher{},
		Verifiers: []ingest.Verifier{&stubVerifier{name: "jira"}, &stubVerifier{name: "github"}},
		Promoter:  promoter,
	})
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if result.Discovered != 2 {
		t.Errorf("discovered = %d, want 2", result.Discovered)
	}
	if result.Matched != 2 {
		t.Errorf("matched = %d, want 2", result.Matched)
	}
	if result.Verified != 2 {
		t.Errorf("verified = %d, want 2", result.Verified)
	}
	if result.Promoted != 2 {
		t.Errorf("promoted = %d, want 2", result.Promoted)
	}

	t.Logf("pipeline: discovered=%d matched=%d verified=%d promoted=%d",
		result.Discovered, result.Matched, result.Verified, result.Promoted)
}

func TestDatasetPipeline_VerifierRejects(t *testing.T) {
	manifest := &ingest.DatasetManifest{
		Kind:    "dataset",
		Version: "v1",
		Metadata: ingest.ManifestMetadata{Scenario: "test"},
		Output:   ingest.ManifestOutput{Scenario: "out.yaml"},
	}

	rejectVerifier := &rejectVerifier{}
	promoter := &stubPromoter{}

	result, err := ingest.RunPipeline(context.Background(), manifest, &ingest.PipelineOpts{
		Source:    &stubSource{records: []ingest.Record{{ID: "F1", DedupKey: "k1", Fields: map[string]any{}}}},
		Matcher:   &stubMatcher{},
		Verifiers: []ingest.Verifier{rejectVerifier},
		Promoter:  promoter,
	})
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if result.Verified != 0 {
		t.Errorf("verified = %d, want 0 (verifier should reject)", result.Verified)
	}
	if result.Promoted != 0 {
		t.Errorf("promoted = %d, want 0", result.Promoted)
	}
}

type rejectVerifier struct{}

func (v *rejectVerifier) Name() string { return "reject" }
func (v *rejectVerifier) Verify(_ context.Context, _ ingest.Record) (ingest.VerifyResult, error) {
	return ingest.VerifyResult{Verified: false, Reason: "jira ticket still open"}, nil
}

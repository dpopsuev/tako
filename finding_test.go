package framework

import (
	"testing"
)

func TestFindingSeverity_Constants(t *testing.T) {
	if FindingInfo != "info" {
		t.Errorf("FindingInfo = %q, want %q", FindingInfo, "info")
	}
	if FindingWarning != "warning" {
		t.Errorf("FindingWarning = %q, want %q", FindingWarning, "warning")
	}
	if FindingError != "error" {
		t.Errorf("FindingError = %q, want %q", FindingError, "error")
	}
}

func TestSeverityAtOrAbove(t *testing.T) {
	tests := []struct {
		have      FindingSeverity
		threshold FindingSeverity
		want      bool
	}{
		{FindingInfo, FindingInfo, true},
		{FindingWarning, FindingInfo, true},
		{FindingError, FindingInfo, true},
		{FindingInfo, FindingWarning, false},
		{FindingWarning, FindingWarning, true},
		{FindingError, FindingWarning, true},
		{FindingInfo, FindingError, false},
		{FindingWarning, FindingError, false},
		{FindingError, FindingError, true},
	}
	for _, tt := range tests {
		got := SeverityAtOrAbove(tt.have, tt.threshold)
		if got != tt.want {
			t.Errorf("SeverityAtOrAbove(%q, %q) = %v, want %v", tt.have, tt.threshold, got, tt.want)
		}
	}
}

func TestFinding_Construction(t *testing.T) {
	f := Finding{
		Severity: FindingError,
		Domain:   "security.auth",
		Source:   "auth-enforcer",
		NodeName: "login",
		Message:  "credentials exposed in artifact",
		Evidence: map[string]any{"field": "password"},
	}

	if f.Severity != FindingError {
		t.Errorf("Severity = %q, want %q", f.Severity, FindingError)
	}
	if f.Domain != "security.auth" {
		t.Errorf("Domain = %q, want %q", f.Domain, "security.auth")
	}
	if f.Evidence["field"] != "password" {
		t.Errorf("Evidence[field] = %v, want %q", f.Evidence["field"], "password")
	}
}

// findingStubArtifact is shared by several root-level test files
// (finding_hook_test.go, finding_expr_test.go, finding_parallel_test.go).
type findingStubArtifact struct {
	typ        string
	confidence float64
	raw        any
}

func (s *findingStubArtifact) Type() string        { return s.typ }
func (s *findingStubArtifact) Confidence() float64 { return s.confidence }
func (s *findingStubArtifact) Raw() any             { return s.raw }

package ingest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DatasetManifest is the consumer-facing configuration for a dataset pipeline.
// Parsed from dataset.yaml in the consumer's repo.
type DatasetManifest struct {
	Kind     string           `yaml:"kind"`
	Version  string           `yaml:"version"`
	Metadata ManifestMetadata `yaml:"metadata"`

	Sources      map[string]ManifestSource   `yaml:"sources"`
	Verification map[string]ManifestVerifier `yaml:"verification"`
	Matching     ManifestMatching            `yaml:"matching"`
	Output       ManifestOutput              `yaml:"output"`
	Schedule     ManifestSchedule            `yaml:"schedule"`
}

// ManifestMetadata identifies the dataset.
type ManifestMetadata struct {
	Name        string `yaml:"name"`
	Scenario    string `yaml:"scenario"`
	Description string `yaml:"description"`
}

// ManifestSource defines a data source (e.g. ReportPortal).
type ManifestSource struct {
	Module string         `yaml:"module"`
	Config map[string]any `yaml:"config"`
	Filter map[string]any `yaml:"filter,omitempty"`
}

// ManifestVerifier defines a verification module (e.g. Jira, GitHub).
type ManifestVerifier struct {
	Module string             `yaml:"module"`
	Config map[string]any     `yaml:"config"`
	Rules  []VerificationRule `yaml:"rules"`
}

// VerificationRule maps a ground truth field to a check.
type VerificationRule struct {
	Field string `yaml:"field"`
	Check string `yaml:"check"`
}

// ManifestMatching configures symptom matching.
type ManifestMatching struct {
	Heuristics           string  `yaml:"heuristics"`
	KeywordThreshold     int     `yaml:"keyword_threshold"`
	AutoPromoteThreshold float64 `yaml:"auto_promote_threshold"`
}

// ManifestOutput defines where promoted cases are written.
type ManifestOutput struct {
	Scenario string `yaml:"scenario"`
}

// ManifestSchedule controls when the pipeline runs.
type ManifestSchedule struct {
	Mode string `yaml:"mode"`
	Cron string `yaml:"cron,omitempty"`
}

// LoadDatasetManifest reads and parses a dataset.yaml file.
func LoadDatasetManifest(path string) (*DatasetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load dataset manifest: %w", err)
	}
	return ParseDatasetManifest(data)
}

// ParseDatasetManifest parses dataset.yaml bytes.
func ParseDatasetManifest(data []byte) (*DatasetManifest, error) {
	var m DatasetManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse dataset manifest: %w", err)
	}
	if m.Kind != "dataset" {
		return nil, fmt.Errorf("%w: %q", ErrDatasetManifestKindMustBeDatasetGot, m.Kind)
	}
	if m.Metadata.Scenario == "" {
		return nil, ErrDatasetManifestMetadataScenarioIsRequired
	}
	return &m, nil
}

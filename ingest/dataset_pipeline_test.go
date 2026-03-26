package ingest_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// RED PHASE: This test defines the ideal dataset.yaml shape.
// It parses the consumer's dataset.yaml and verifies the schema.
// Currently: parses but can't execute (no pipeline runner exists).

// DatasetManifest is the desired schema for dataset.yaml.
// This struct IS the spec — when this test passes end-to-end,
// the pipeline works.
type DatasetManifest struct {
	Kind     string `yaml:"kind"`
	Version  string `yaml:"version"`
	Metadata struct {
		Name        string `yaml:"name"`
		Scenario    string `yaml:"scenario"`
		Description string `yaml:"description"`
	} `yaml:"metadata"`

	Sources map[string]struct {
		Module string         `yaml:"module"`
		Config map[string]any `yaml:"config"`
		Filter map[string]any `yaml:"filter,omitempty"`
	} `yaml:"sources"`

	Verification map[string]struct {
		Module string         `yaml:"module"`
		Config map[string]any `yaml:"config"`
		Rules  []struct {
			Field string `yaml:"field"`
			Check string `yaml:"check"`
		} `yaml:"rules"`
	} `yaml:"verification"`

	Matching struct {
		Heuristics            string  `yaml:"heuristics"`
		KeywordThreshold      int     `yaml:"keyword_threshold"`
		AutoPromoteThreshold  float64 `yaml:"auto_promote_threshold"`
	} `yaml:"matching"`

	Output struct {
		Scenario string `yaml:"scenario"`
	} `yaml:"output"`

	Schedule struct {
		Mode string `yaml:"mode"`
	} `yaml:"schedule"`
}

func TestDatasetManifest_ParsesConsumerYAML(t *testing.T) {
	// RED: Parse the ideal dataset.yaml from asterisk.
	// This validates the SHAPE is right.
	data, err := os.ReadFile("../testdata/dataset.yaml")
	if err != nil {
		// Try the actual asterisk location for local dev.
		home, _ := os.UserHomeDir()
		data, err = os.ReadFile(home + "/Workspace/asterisk/dataset.yaml")
		if err != nil {
			t.Skip("dataset.yaml not found — run from origami root or set up testdata")
		}
	}

	var m DatasetManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse dataset.yaml: %v", err)
	}

	// Validate shape.
	if m.Kind != "dataset" {
		t.Errorf("kind = %q, want dataset", m.Kind)
	}
	if m.Version != "v1" {
		t.Errorf("version = %q, want v1", m.Version)
	}
	if m.Metadata.Scenario == "" {
		t.Error("metadata.scenario is required")
	}

	// Sources must include reportportal.
	if _, ok := m.Sources["reportportal"]; !ok {
		t.Error("sources must include reportportal")
	}

	// Verification must include jira and github.
	if _, ok := m.Verification["jira"]; !ok {
		t.Error("verification must include jira")
	}
	if _, ok := m.Verification["github"]; !ok {
		t.Error("verification must include github")
	}

	// Output must point to a scenario file.
	if m.Output.Scenario == "" {
		t.Error("output.scenario is required")
	}

	t.Logf("dataset manifest: name=%s scenario=%s sources=%d verifiers=%d",
		m.Metadata.Name, m.Metadata.Scenario, len(m.Sources), len(m.Verification))
}

func TestDatasetPipeline_RunSync(t *testing.T) {
	// RED: This test will fail until the pipeline runner exists.
	// It should:
	//   1. Load dataset.yaml
	//   2. Discover failures from RP source
	//   3. Match against heuristics
	//   4. Verify with Jira + GitHub
	//   5. Promote to scenario YAML

	t.Skip("RED: pipeline runner not implemented — needs ingest.RunPipeline()")

	// When implemented, this will look like:
	// m := loadManifest(t, "dataset.yaml")
	// result, err := ingest.RunPipeline(ctx, m)
	// if err != nil { t.Fatal(err) }
	// if result.Promoted == 0 { t.Error("expected at least one promoted case") }
}

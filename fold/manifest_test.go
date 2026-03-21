package fold

import (
	"strings"
	"testing"
)

func TestParseManifest_Minimal(t *testing.T) {
	data := []byte(`
name: test-tool
description: A test tool
version: "1.0"
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-tool" {
		t.Errorf("name = %q, want test-tool", m.Name)
	}
	if m.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", m.Version)
	}
}

func TestParseManifest_DomainServe(t *testing.T) {
	data := []byte(`
name: asterisk
version: "1.0"
domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.DomainServe == nil {
		t.Fatal("domain_serve is nil")
	}
	if m.DomainServe.Port != 9300 {
		t.Errorf("port = %d, want 9300", m.DomainServe.Port)
	}
	if m.DomainServe.Assets == nil {
		t.Fatal("assets is nil")
	}
}

func TestParseManifest_MissingName(t *testing.T) {
	data := []byte(`description: no name`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseManifest_Assets(t *testing.T) {
	data := []byte(`
name: asterisk
version: "1.0"
domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
      calibration: circuits/calibration.yaml
    prompts:
      recall: prompts/recall/judge-similarity.md
      triage: prompts/triage/classify-symptoms.md
    schemas:
      recall: schemas/rca/F0_RECALL.yaml
    scenarios:
      ptp-mock: scenarios/ptp-mock.yaml
    scorecards:
      rca: scorecards/rca.yaml
    reports:
      rca: reports/rca-report.yaml
    sources:
      ptp: sources/ptp.yaml
    files:
      vocabulary: vocabulary.yaml
      heuristics: heuristics.yaml
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.DomainServe == nil {
		t.Fatal("domain_serve is nil")
	}
	a := m.DomainServe.Assets
	if a == nil {
		t.Fatal("assets is nil")
	}
	if got := a.Circuits["rca"]; got != "circuits/rca.yaml" {
		t.Errorf("circuits.rca = %q, want circuits/rca.yaml", got)
	}
	if got := a.Prompts["recall"]; got != "prompts/recall/judge-similarity.md" {
		t.Errorf("prompts.recall = %q", got)
	}
	if got := a.Files["vocabulary"]; got != "vocabulary.yaml" {
		t.Errorf("files.vocabulary = %q, want vocabulary.yaml", got)
	}
	if got := a.Files["heuristics"]; got != "heuristics.yaml" {
		t.Errorf("files.heuristics = %q, want heuristics.yaml", got)
	}
}

func TestParseManifest_NoAssets(t *testing.T) {
	data := []byte(`
name: bad
version: "1.0"
domain_serve:
  port: 9300
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing assets")
	}
	if !strings.Contains(err.Error(), "assets is required") {
		t.Errorf("error = %q, want mention of assets is required", err.Error())
	}
}

func TestAssetMap_AllPaths(t *testing.T) {
	a := &AssetMap{
		Circuits: map[string]string{
			"rca":         "circuits/rca.yaml",
			"calibration": "circuits/calibration.yaml",
		},
		Prompts: map[string]string{
			"recall": "prompts/recall/judge-similarity.md",
		},
		Schemas: map[string]string{
			"recall": "schemas/rca/F0_RECALL.yaml",
		},
		Files: map[string]string{
			"vocabulary": "vocabulary.yaml",
			"heuristics": "heuristics.yaml",
		},
	}

	paths := a.AllPaths()

	want := []string{
		"circuits/calibration.yaml",
		"circuits/rca.yaml",
		"heuristics.yaml",
		"prompts/recall/judge-similarity.md",
		"schemas/rca/F0_RECALL.yaml",
		"vocabulary.yaml",
	}
	if len(paths) != len(want) {
		t.Fatalf("AllPaths() = %v, want %v", paths, want)
	}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("AllPaths()[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestAssetMap_AllPaths_Dedup(t *testing.T) {
	a := &AssetMap{
		Circuits: map[string]string{"a": "shared.yaml"},
		Prompts:  map[string]string{"b": "shared.yaml"},
	}
	paths := a.AllPaths()
	if len(paths) != 1 {
		t.Fatalf("expected 1 deduplicated path, got %v", paths)
	}
	if paths[0] != "shared.yaml" {
		t.Errorf("path = %q, want shared.yaml", paths[0])
	}
}

func TestAssetMap_Sections(t *testing.T) {
	a := &AssetMap{
		Circuits: map[string]string{"rca": "circuits/rca.yaml"},
		Prompts:  map[string]string{"recall": "prompts/recall.md"},
		Files:    map[string]string{"vocab": "vocab.yaml"},
	}
	sections := a.Sections()
	if _, ok := sections["circuits"]; !ok {
		t.Error("missing circuits section")
	}
	if _, ok := sections["prompts"]; !ok {
		t.Error("missing prompts section")
	}
	if _, ok := sections["files"]; ok {
		t.Error("files should not appear in Sections()")
	}
}

func TestAssetMap_ScalarFiles(t *testing.T) {
	a := &AssetMap{
		Files: map[string]string{
			"vocabulary": "vocabulary.yaml",
			"heuristics": "heuristics.yaml",
		},
	}
	files := a.ScalarFiles()
	if files["vocabulary"] != "vocabulary.yaml" {
		t.Errorf("vocabulary = %q", files["vocabulary"])
	}
	if files["heuristics"] != "heuristics.yaml" {
		t.Errorf("heuristics = %q", files["heuristics"])
	}
}

func TestAssetMap_ScalarFiles_Empty(t *testing.T) {
	a := &AssetMap{}
	if files := a.ScalarFiles(); files != nil {
		t.Errorf("expected nil for empty files, got %v", files)
	}
}

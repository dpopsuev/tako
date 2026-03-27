package fold

import (
	"os"
	"strings"
	"testing"
)

func loadFixtureManifest(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/manifests/" + name + ".yaml")
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func TestParseManifest_Minimal(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "minimal"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-tool" {
		t.Errorf("name = %q, want test-tool", m.Name)
	}
}

func TestParseManifest_DomainServe(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "domain-serve"))
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
	_, err := ParseManifest(loadFixtureManifest(t, "missing-name"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseManifest_MissingAPIVersion(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "missing-apiversion"))
	if err == nil {
		t.Fatal("expected error for missing apiVersion")
	}
}

func TestParseManifest_WrongKind(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "wrong-kind"))
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestParseManifest_Assets(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	a := m.DomainServe.Assets
	if a == nil {
		t.Fatal("assets is nil")
	}
	if got := a.Circuits["rca"]; got != "circuits/rca.yaml" {
		t.Errorf("circuits.rca = %q", got)
	}
	if got := a.Prompts["recall"]; got != "prompts/recall/judge-similarity.md" {
		t.Errorf("prompts.recall = %q", got)
	}
	if got := a.Files["vocabulary"]; got != "vocabulary.yaml" {
		t.Errorf("files.vocabulary = %q", got)
	}
}

func TestParseManifest_NoAssets(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "no-assets"))
	if err == nil {
		t.Fatal("expected error for missing assets")
	}
	if !strings.Contains(err.Error(), "assets is required") {
		t.Errorf("error = %q, want mention of assets is required", err.Error())
	}
}

func TestAssetMap_AllPaths(t *testing.T) {
	a := &AssetMap{
		Circuits: map[string]string{"rca": "circuits/rca.yaml", "calibration": "circuits/calibration.yaml"},
		Prompts:  map[string]string{"recall": "prompts/recall/judge-similarity.md"},
		Schemas:  map[string]string{"recall": "schemas/rca/F0_RECALL.yaml"},
		Files:    map[string]string{"vocabulary": "vocabulary.yaml", "heuristics": "heuristics.yaml"},
	}
	paths := a.AllPaths()
	want := []string{
		"circuits/calibration.yaml", "circuits/rca.yaml", "heuristics.yaml",
		"prompts/recall/judge-similarity.md", "schemas/rca/F0_RECALL.yaml", "vocabulary.yaml",
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
	if paths := a.AllPaths(); len(paths) != 1 {
		t.Fatalf("expected 1 deduplicated path, got %v", paths)
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
	if _, ok := sections["files"]; ok {
		t.Error("files should not appear in Sections()")
	}
}

func TestAssetMap_ScalarFiles(t *testing.T) {
	a := &AssetMap{Files: map[string]string{"vocabulary": "vocabulary.yaml"}}
	if files := a.ScalarFiles(); files["vocabulary"] != "vocabulary.yaml" {
		t.Errorf("vocabulary = %q", files["vocabulary"])
	}
}

func TestAssetMap_ScalarFiles_Empty(t *testing.T) {
	a := &AssetMap{}
	if files := a.ScalarFiles(); files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

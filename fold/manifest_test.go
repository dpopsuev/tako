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
	if got := a.Circuits["alpha"]; got != "circuits/alpha.yaml" {
		t.Errorf("circuits.alpha = %q", got)
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
		Circuits: map[string]string{"alpha": "circuits/alpha.yaml", "calibration": "circuits/calibration.yaml"},
		Prompts:  map[string]string{"recall": "prompts/recall/judge-similarity.md"},
		Schemas:  map[string]string{"recall": "schemas/alpha/F0_RECALL.yaml"},
		Files:    map[string]string{"vocabulary": "vocabulary.yaml", "heuristics": "heuristics.yaml"},
	}
	paths := a.AllPaths()
	want := []string{
		"circuits/alpha.yaml", "circuits/calibration.yaml", "heuristics.yaml",
		"prompts/recall/judge-similarity.md", "schemas/alpha/F0_RECALL.yaml", "vocabulary.yaml",
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
		Circuits: map[string]string{"alpha": "circuits/alpha.yaml"},
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

func TestValidateFileKind_MissingKindHeader(t *testing.T) {
	// Create a temp file with no kind: header.
	tmpDir := t.TempDir()
	noKindFile := tmpDir + "/no-kind.yaml"
	if err := os.WriteFile(noKindFile, []byte("name: test\nversion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateFileKind(noKindFile, "Scenario")
	if err == nil {
		t.Fatal("expected error for missing kind header")
	}
	if !strings.Contains(err.Error(), "no kind: header") {
		t.Errorf("error = %q, want mention of missing kind header", err.Error())
	}
}

func TestValidateFileKind_CorrectKind(t *testing.T) {
	tmpDir := t.TempDir()
	goodFile := tmpDir + "/good.yaml"
	if err := os.WriteFile(goodFile, []byte("kind: Scenario\nname: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateFileKind(goodFile, "Scenario"); err != nil {
		t.Errorf("unexpected error for correct kind: %v", err)
	}
}

func TestValidateFileKind_WrongKind(t *testing.T) {
	tmpDir := t.TempDir()
	wrongFile := tmpDir + "/wrong.yaml"
	if err := os.WriteFile(wrongFile, []byte("kind: Tuning\nname: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateFileKind(wrongFile, "Scenario")
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
	if !strings.Contains(err.Error(), "domain kind mismatch") {
		t.Errorf("error = %q, want mention of domain kind mismatch", err.Error())
	}
}

package domainfs_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dpopsuev/origami/domainfs"
)

func newTestAssets() (*domainfs.DomainAssets, fstest.MapFS) {
	mfs := fstest.MapFS{
		"reports/rca-report.yaml":  &fstest.MapFile{Data: []byte("rca: true")},
		"reports/gnd-report.yaml":  &fstest.MapFile{Data: []byte("gnd: true")},
		"prompts/recall/judge.md":  &fstest.MapFile{Data: []byte("You are a judge.")},
		"circuits/rca.yaml":        &fstest.MapFile{Data: []byte("circuit: rca")},
	}
	sections := map[string]map[string]string{
		"reports": {
			"rca-report": "reports/rca-report.yaml",
			"gnd-report": "reports/gnd-report.yaml",
		},
		"prompts": {
			"recall-judge": "prompts/recall/judge.md",
		},
		"circuits": {
			"rca": "circuits/rca.yaml",
		},
	}
	return domainfs.NewDomainAssets(mfs, sections), mfs
}

func TestReadAsset_Valid(t *testing.T) {
	da, _ := newTestAssets()

	data, err := da.ReadAsset("reports", "rca-report")
	if err != nil {
		t.Fatalf("ReadAsset: %v", err)
	}
	if string(data) != "rca: true" {
		t.Errorf("content = %q, want %q", string(data), "rca: true")
	}

	data, err = da.ReadAsset("prompts", "recall-judge")
	if err != nil {
		t.Fatalf("ReadAsset prompts/recall-judge: %v", err)
	}
	if string(data) != "You are a judge." {
		t.Errorf("content = %q, want %q", string(data), "You are a judge.")
	}
}

func TestReadAsset_InvalidKey(t *testing.T) {
	da, _ := newTestAssets()

	_, err := da.ReadAsset("reports", "calibration-report")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}

	msg := err.Error()
	// Must mention the missing key
	if !strings.Contains(msg, "calibration-report") {
		t.Errorf("error should mention missing key, got: %s", msg)
	}
	// Must list available keys
	if !strings.Contains(msg, "rca-report") {
		t.Errorf("error should list available key 'rca-report', got: %s", msg)
	}
	if !strings.Contains(msg, "gnd-report") {
		t.Errorf("error should list available key 'gnd-report', got: %s", msg)
	}
	// Must mention the section
	if !strings.Contains(msg, `section "reports"`) {
		t.Errorf("error should mention section, got: %s", msg)
	}
}

func TestReadAsset_InvalidSection(t *testing.T) {
	da, _ := newTestAssets()

	_, err := da.ReadAsset("heuristics", "rules")
	if err == nil {
		t.Fatal("expected error for invalid section")
	}

	msg := err.Error()
	// Must mention the missing section
	if !strings.Contains(msg, "heuristics") {
		t.Errorf("error should mention missing section, got: %s", msg)
	}
	// Must list available sections
	if !strings.Contains(msg, "reports") {
		t.Errorf("error should list available section 'reports', got: %s", msg)
	}
	if !strings.Contains(msg, "prompts") {
		t.Errorf("error should list available section 'prompts', got: %s", msg)
	}
	if !strings.Contains(msg, "circuits") {
		t.Errorf("error should list available section 'circuits', got: %s", msg)
	}
}

func TestHasAsset(t *testing.T) {
	da, _ := newTestAssets()

	tests := []struct {
		section string
		key     string
		want    bool
	}{
		{"reports", "rca-report", true},
		{"reports", "gnd-report", true},
		{"reports", "calibration-report", false},
		{"prompts", "recall-judge", true},
		{"heuristics", "rules", false},
		{"circuits", "rca", true},
		{"circuits", "gnd", false},
	}

	for _, tt := range tests {
		got := da.HasAsset(tt.section, tt.key)
		if got != tt.want {
			t.Errorf("HasAsset(%q, %q) = %v, want %v", tt.section, tt.key, got, tt.want)
		}
	}
}

func TestReadAsset_NilFS(t *testing.T) {
	da := domainfs.NewDomainAssets(nil, map[string]map[string]string{
		"reports": {"rca-report": "reports/rca-report.yaml"},
	})

	_, err := da.ReadAsset("reports", "rca-report")
	if err == nil {
		t.Fatal("expected error for nil FS")
	}
	if !strings.Contains(err.Error(), "nil filesystem") {
		t.Errorf("error should mention nil filesystem, got: %s", err.Error())
	}
}

func TestFS_ReturnsUnderlying(t *testing.T) {
	da, _ := newTestAssets()

	got := da.FS()
	if got == nil {
		t.Fatal("FS() should return non-nil filesystem")
	}
	// Verify it's usable: read a known file through the returned FS.
	data, err := fs.ReadFile(got, "reports/rca-report.yaml")
	if err != nil {
		t.Fatalf("ReadFile via FS(): %v", err)
	}
	if string(data) != "rca: true" {
		t.Errorf("content via FS() = %q, want %q", string(data), "rca: true")
	}
}

func TestFS_NilReturnsNil(t *testing.T) {
	da := domainfs.NewDomainAssets(nil, nil)
	if da.FS() != nil {
		t.Error("FS() should return nil for nil filesystem")
	}
}

func TestSection_ReturnsCopy(t *testing.T) {
	da, _ := newTestAssets()

	sec := da.Section("reports")
	if sec == nil {
		t.Fatal("Section('reports') returned nil")
	}
	if len(sec) != 2 {
		t.Fatalf("Section('reports') has %d entries, want 2", len(sec))
	}
	if sec["rca-report"] != "reports/rca-report.yaml" {
		t.Errorf("reports[rca-report] = %q", sec["rca-report"])
	}

	// Mutating the copy should not affect the original.
	sec["evil"] = "evil.yaml"
	sec2 := da.Section("reports")
	if _, ok := sec2["evil"]; ok {
		t.Error("mutation of Section() return value leaked into DomainAssets")
	}
}

func TestSection_UnknownReturnsNil(t *testing.T) {
	da, _ := newTestAssets()

	if got := da.Section("nonexistent"); got != nil {
		t.Errorf("Section('nonexistent') = %v, want nil", got)
	}
}

func TestNewDomainAssets_DefensiveCopy(t *testing.T) {
	sections := map[string]map[string]string{
		"reports": {"rca-report": "reports/rca-report.yaml"},
	}
	mfs := fstest.MapFS{
		"reports/rca-report.yaml": &fstest.MapFile{Data: []byte("rca: true")},
	}
	da := domainfs.NewDomainAssets(mfs, sections)

	// Mutate the original map after construction.
	sections["reports"]["injected"] = "injected.yaml"
	sections["new-section"] = map[string]string{"x": "y"}

	if da.HasAsset("reports", "injected") {
		t.Error("mutation of input sections leaked into DomainAssets")
	}
	if da.HasAsset("new-section", "x") {
		t.Error("new section added to input leaked into DomainAssets")
	}
}

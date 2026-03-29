package fold

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockResolver tracks calls and returns configurable paths.
type mockResolver struct {
	paths map[string]string
	calls []string
}

func (r *mockResolver) FindLocalModule(modPath string) string {
	r.calls = append(r.calls, modPath)
	return r.paths[modPath]
}

func TestCreateWiredBuildModule_DefaultNoReplace(t *testing.T) {
	// RED: Default fold (local=false) must NOT inject replace directives.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule:                     "/home/user/Workspace/origami",
			"github.com/dpopsuev/origami-rca": "/home/user/Workspace/origami-rca",
		},
	}
	g := &ResolvedGraph{
		Imports: []ImportEntry{{Path: "github.com/dpopsuev/origami-rca"}},
	}

	err := createWiredBuildModule(tmpDir, "test", resolver, g, false)
	if err != nil {
		t.Fatalf("createWiredBuildModule: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "replace") {
		t.Errorf("default fold should NOT contain replace directives, got:\n%s", content)
	}
}

func TestCreateWiredBuildModule_LocalEnablesReplace(t *testing.T) {
	// RED: --local flag must enable replace directives.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule:                     "/home/user/Workspace/origami",
			"github.com/dpopsuev/origami-rca": "/home/user/Workspace/origami-rca",
		},
	}
	g := &ResolvedGraph{
		Imports: []ImportEntry{{Path: "github.com/dpopsuev/origami-rca"}},
	}

	err := createWiredBuildModule(tmpDir, "test", resolver, g, true)
	if err != nil {
		t.Fatalf("createWiredBuildModule: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "replace "+origamiModule) {
		t.Errorf("--local should inject origami replace, got:\n%s", content)
	}
	if !strings.Contains(content, "replace github.com/dpopsuev/origami-rca") {
		t.Errorf("--local should inject rca replace, got:\n%s", content)
	}
}

func TestCreateWiredBuildModule_ResolverNotCalledWithoutLocal(t *testing.T) {
	// RED: When local=false, FindLocalModule must NOT be called.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{origamiModule: "/somewhere"},
	}

	err := createWiredBuildModule(tmpDir, "test", resolver, nil, false)
	if err != nil {
		t.Fatalf("createWiredBuildModule: %v", err)
	}

	if len(resolver.calls) > 0 {
		t.Errorf("FindLocalModule should not be called when local=false, got %d calls: %v",
			len(resolver.calls), resolver.calls)
	}
}

func TestCreateDomainServeBuildModule_DefaultNoReplace(t *testing.T) {
	// RED: Domain-serve path must also respect local flag.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{origamiModule: "/home/user/Workspace/origami"},
	}

	err := createDomainServeBuildModule(tmpDir, "test", resolver, false)
	if err != nil {
		t.Fatalf("createDomainServeBuildModule: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "replace") {
		t.Errorf("default domain-serve should NOT contain replace directives, got:\n%s", content)
	}
}

func TestCreateDomainServeBuildModule_LocalEnablesReplace(t *testing.T) {
	// RED: --local on domain-serve must inject replace.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{origamiModule: "/home/user/Workspace/origami"},
	}

	err := createDomainServeBuildModule(tmpDir, "test", resolver, true)
	if err != nil {
		t.Fatalf("createDomainServeBuildModule: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "replace "+origamiModule) {
		t.Errorf("--local should inject origami replace, got:\n%s", content)
	}
}

func TestCreateWiredBuildModule_LocalPrintsWarning(t *testing.T) {
	// Capture stderr to verify WARNING is printed.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule: "/home/user/Workspace/origami",
		},
	}

	// Redirect stderr to a buffer.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := createWiredBuildModule(tmpDir, "test", resolver, nil, true)
	if err != nil {
		t.Fatalf("createWiredBuildModule: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	stderr := buf.String()
	if !strings.Contains(stderr, "WARNING: using local module") {
		t.Errorf("expected WARNING on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, origamiModule) {
		t.Errorf("WARNING should mention module path, got: %q", stderr)
	}
}

func TestCreateWiredBuildModule_NoWarningWithoutLocal(t *testing.T) {
	// Verify NO warning when local=false.
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule: "/home/user/Workspace/origami",
		},
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := createWiredBuildModule(tmpDir, "test", resolver, nil, false)
	if err != nil {
		t.Fatalf("createWiredBuildModule: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	if buf.Len() > 0 {
		t.Errorf("no stderr expected without --local, got: %q", buf.String())
	}
}

func TestCreateDomainServeBuildModule_LocalPrintsWarning(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule: "/home/user/Workspace/origami",
		},
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := createDomainServeBuildModule(tmpDir, "test", resolver, true)
	if err != nil {
		t.Fatalf("createDomainServeBuildModule: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	stderr := buf.String()
	if !strings.Contains(stderr, "WARNING: using local module") {
		t.Errorf("expected WARNING on stderr, got: %q", stderr)
	}
}

func TestLocalFlag_SimulatedRemoteFailure(t *testing.T) {
	// Simulate the origami-rca incident:
	// A module exists locally but not in the go.mod when local=false.
	// Without --local: go.mod has NO replace → go mod tidy would fail (module not on proxy).
	// With --local: go.mod has replace → build succeeds.

	fakeModule := "github.com/example/fake-schematic"

	// WITHOUT --local: no replace directive
	tmpDir1 := t.TempDir()
	resolver := &mockResolver{
		paths: map[string]string{
			origamiModule: "/home/user/Workspace/origami",
			fakeModule:    "/home/user/Workspace/fake-schematic",
		},
	}
	g := &ResolvedGraph{
		Imports: []ImportEntry{{Path: fakeModule}},
	}

	err := createWiredBuildModule(tmpDir1, "test", resolver, g, false)
	if err != nil {
		t.Fatalf("createWiredBuildModule (no local): %v", err)
	}
	data1, _ := os.ReadFile(filepath.Join(tmpDir1, "go.mod"))
	content1 := string(data1)

	if strings.Contains(content1, "replace") {
		t.Errorf("without --local: go.mod should NOT have replace directives\n%s", content1)
	}
	// The require block has v0.0.0 — go mod tidy would fail because
	// github.com/example/fake-schematic@v0.0.0 doesn't exist on proxy.
	if !strings.Contains(content1, fakeModule+" v0.0.0") {
		t.Errorf("should require fake module at v0.0.0, got:\n%s", content1)
	}

	// WITH --local: replace directive present → build would succeed
	tmpDir2 := t.TempDir()
	err = createWiredBuildModule(tmpDir2, "test", resolver, g, true)
	if err != nil {
		t.Fatalf("createWiredBuildModule (local): %v", err)
	}
	data2, _ := os.ReadFile(filepath.Join(tmpDir2, "go.mod"))
	content2 := string(data2)

	if !strings.Contains(content2, "replace "+fakeModule+" => /home/user/Workspace/fake-schematic") {
		t.Errorf("with --local: should have replace for fake module, got:\n%s", content2)
	}
}

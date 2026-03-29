package fold

import (
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

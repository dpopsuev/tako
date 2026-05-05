package rehearsal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func repoRoot() string {
	wd, _ := os.Getwd()
	for d := wd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	return wd
}

func buildTako(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "tako")
	root := repoRoot()
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/tako")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build tako: %s\n%s", err, out)
	}
	return binary
}

func TestSubprocessActor_Compiles(t *testing.T) {
	var _ Actor = &SubprocessActor{}
}

func TestSubprocessActor_Version(t *testing.T) {
	binary := buildTako(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako version: %v\n%s", err, out)
	}
	t.Logf("tako version: %s", out)
}

func TestSubprocessActor_MissingProvider(t *testing.T) {
	binary := buildTako(t)

	actor := &SubprocessActor{
		Binary: binary,
		Env:    []string{"ANTHROPIC_API_KEY=", "CLOUD_ML_REGION=", "GOOGLE_API_KEY=", "OPENROUTER_API_KEY="},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := actor.Run(ctx, "hello")
	if err == nil {
		t.Error("expected error when no provider configured")
	}
	t.Logf("Expected error: %v", err)
}

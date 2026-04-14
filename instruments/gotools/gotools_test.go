package gotools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

func origamiRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("origami root not found at %s", root)
	}
	return root
}

// Contract: BuildTransformer returns typed *sdlctype.BuildResult.
func TestBuild_ReturnsTypedResult(t *testing.T) {
	root := origamiRoot(t)
	tx := NewBuildTransformer(root)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.TransformerContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	br, ok := result.(*sdlctype.BuildResult)
	if !ok {
		t.Fatalf("expected *sdlctype.BuildResult, got %T", result)
	}

	if !br.Pass {
		t.Errorf("Origami build failed: %s", br.Output)
	}
	t.Logf("Build pass=%v output_len=%d", br.Pass, len(br.Output))
}

// Contract: TestTransformer returns typed *sdlctype.TestResult.
// Runs only engine/trace/ (fast, ~0.01s) to validate the instrument.
func TestTest_ReturnsTypedResult(t *testing.T) {
	root := origamiRoot(t)
	tx := NewTestTransformer(root, WithPackages("./engine/trace/"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.TransformerContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	tr, ok := result.(*sdlctype.TestResult)
	if !ok {
		t.Fatalf("expected *sdlctype.TestResult, got %T", result)
	}

	if !tr.Pass {
		t.Errorf("engine/trace tests failed: %d/%d failed\n%s", tr.Failed, tr.Total, tr.Output)
	}
	t.Logf("Test pass=%v total=%d failed=%d", tr.Pass, tr.Total, tr.Failed)
}

// Contract: same interface as stub — Liskov substitution.
func TestBuildContract_MatchesStub(t *testing.T) {
	root := origamiRoot(t)
	transformers := map[string]engine.Transformer{
		"stub": engine.TransformerFunc("build", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
			return &sdlctype.BuildResult{Pass: true}, nil
		}),
		"real": NewBuildTransformer(root),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for name, tx := range transformers {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.TransformerContext{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			br, ok := result.(*sdlctype.BuildResult)
			if !ok {
				t.Fatalf("expected *sdlctype.BuildResult, got %T", result)
			}
			// Both must have the Pass field.
			_ = br.Pass
		})
	}
}

// Contract: same interface as stub — Liskov substitution.
func TestTestContract_MatchesStub(t *testing.T) {
	root := origamiRoot(t)
	transformers := map[string]engine.Transformer{
		"stub": engine.TransformerFunc("test", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
			return &sdlctype.TestResult{Pass: true}, nil
		}),
		"real": NewTestTransformer(root, WithPackages("./engine/trace/")),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	for name, tx := range transformers {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.TransformerContext{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			tr, ok := result.(*sdlctype.TestResult)
			if !ok {
				t.Fatalf("expected *sdlctype.TestResult, got %T", result)
			}
			_ = tr.Pass
			_ = tr.Total
		})
	}
}

// Build on a bad path should return Pass=false, not an error.
func TestBuild_FailingBuild_ReturnsPassFalse(t *testing.T) {
	tx := NewBuildTransformer(t.TempDir()) // empty dir, no go.mod

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.TransformerContext{})
	if err != nil {
		t.Fatalf("Transform should not error: %v", err)
	}

	br := result.(*sdlctype.BuildResult)
	if br.Pass {
		t.Error("expected Pass=false for empty directory build")
	}
}

func TestParseTestCounts(t *testing.T) {
	output := `ok  	github.com/foo/bar	0.5s
ok  	github.com/foo/baz	1.2s
FAIL	github.com/foo/broken	0.3s
ok  	github.com/foo/qux	0.1s`

	total, failed := parseTestCounts(output)
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

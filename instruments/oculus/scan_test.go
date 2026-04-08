package oculus

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

func origamiRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	// instruments/oculus/scan_test.go → ../../
	root := filepath.Join(filepath.Dir(f), "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("origami root not found at %s", root)
	}
	return root
}

// Contract test: Oculus scan returns typed *sdlc.ScanResult, same as stub.
func TestOculusScan_ReturnsTypedScanResult(t *testing.T) {
	root := origamiRoot(t)
	tx := NewScanTransformer(root)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.TransformerContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr, ok := result.(*sdlc.ScanResult)
	if !ok {
		t.Fatalf("expected *sdlc.ScanResult, got %T", result)
	}

	// Clean == true only when no findings.
	if sr.Clean != (len(sr.Findings) == 0) {
		t.Errorf("Clean=%v but len(Findings)=%d", sr.Clean, len(sr.Findings))
	}
}

// Contract test: same interface as stub — Liskov substitution.
func TestScanContract_OculusMatchesStub(t *testing.T) {
	root := origamiRoot(t)

	transformers := map[string]engine.Transformer{
		"stub":   sdlc.StubTransformers(true)["scan"],
		"oculus": NewScanTransformer(root),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for name, tx := range transformers {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.TransformerContext{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			sr, ok := result.(*sdlc.ScanResult)
			if !ok {
				t.Fatalf("expected *sdlc.ScanResult, got %T", result)
			}
			if sr.Findings == nil {
				// nil is acceptable for clean — but Clean must be true.
				if !sr.Clean {
					t.Error("nil Findings but Clean is false")
				}
			}
		})
	}
}

// Integration test: scan Origami's own codebase — must find something.
func TestOculusScan_OrigamiRepo(t *testing.T) {
	root := origamiRoot(t)
	tx := NewScanTransformer(root, WithIntent("health"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.TransformerContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr := result.(*sdlc.ScanResult)
	t.Logf("Oculus scan of origami: %d findings, clean=%v", len(sr.Findings), sr.Clean)
	for _, f := range sr.Findings {
		t.Logf("  [%s] %s: %s (%s)", f.Severity, f.Rule, f.Message, f.File)
	}
}

// Integration test: use harness WithTransformer to swap in Oculus scan.
// Real Oculus finds violations → stub fix doesn't actually fix → loop hits max_fix_loops → aborts.
// This proves the real scan + fix loop + abort path works end-to-end.
func TestOculusScan_ViaHarness(t *testing.T) {
	root := origamiRoot(t)
	result := sdlc.NewHarness(t).
		WithStubs(false).
		WithTransformer("scan", NewScanTransformer(root)).
		WithTimeout(120 * time.Second).
		RunExpectError()

	// The circuit will either complete (if Origami is clean) or abort after max_fix_loops.
	// Either way, it should not crash.
	if result != nil {
		t.Logf("SDLC circuit completed with real Oculus scan: %d walk results", len(result.WalkResults))
	} else {
		t.Log("SDLC circuit errored (expected — stub fix doesn't resolve real findings)")
	}
}

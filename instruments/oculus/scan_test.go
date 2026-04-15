package oculus

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

// Contract test: Oculus scan returns typed *sdlctype.ScanResult, same as stub.
func TestOculusScan_ReturnsTypedScanResult(t *testing.T) {
	root := origamiRoot(t)
	tx := NewScanTransformer(root)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.InstrumentContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr, ok := result.(*sdlctype.ScanResult)
	if !ok {
		t.Fatalf("expected *sdlctype.ScanResult, got %T", result)
	}

	if sr.Clean != (len(sr.Findings) == 0) {
		t.Errorf("Clean=%v but len(Findings)=%d", sr.Clean, len(sr.Findings))
	}
}

// Contract test: stub and real both return *sdlctype.ScanResult — Liskov.
func TestScanContract_OculusMatchesStub(t *testing.T) {
	root := origamiRoot(t)

	// Inline stub — avoids importing simulate/sdlc (import cycle).
	stubScan := engine.InstrumentFunc("scan", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.ScanResult{Clean: true}, nil
	})

	transformers := map[string]engine.Instrument{
		"stub":   stubScan,
		"oculus": NewScanTransformer(root),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for name, tx := range transformers {
		t.Run(name, func(t *testing.T) {
			result, err := tx.Transform(ctx, &engine.InstrumentContext{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			sr, ok := result.(*sdlctype.ScanResult)
			if !ok {
				t.Fatalf("expected *sdlctype.ScanResult, got %T", result)
			}
			if sr.Findings == nil {
				if !sr.Clean {
					t.Error("nil Findings but Clean is false")
				}
			}
		})
	}
}

// Integration test: scan Origami's own codebase.
func TestOculusScan_OrigamiRepo(t *testing.T) {
	root := origamiRoot(t)
	tx := NewScanTransformer(root, WithIntent("health"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := tx.Transform(ctx, &engine.InstrumentContext{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr := result.(*sdlctype.ScanResult)
	t.Logf("Oculus scan of origami: %d findings, clean=%v", len(sr.Findings), sr.Clean)
	for _, f := range sr.Findings {
		t.Logf("  [%s] %s: %s (%s)", f.Severity, f.Rule, f.Message, f.File)
	}
}

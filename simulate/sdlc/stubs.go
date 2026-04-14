package sdlc

import (
	"context"
	"sync/atomic"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// StubTransformers returns a transformer registry with stubs for every
// SDLC circuit node. Each stub returns a typed struct, not map[string]any.
// The clean parameter controls whether the scan stub reports a clean codebase
// (true → clean path, false → fix loop path).
func StubTransformers(clean bool) engine.TransformerRegistry {
	// Scan alternates: first call returns findings (or clean), subsequent
	// calls after a fix loop return clean. This simulates the self-reinforcing
	// loop: scan→fix→build→test→deploy→validate→scan(now clean)→harden→release.
	scanCalls := &atomic.Int32{}

	return engine.TransformerRegistry{
		"scan":          stubScan(clean, scanCalls),
		"fix":           stubFix(),
		"build":         stubBuild(),
		"test":          stubTest(),
		"self-review":   stubSelfReview(),
		"deploy-canary": stubDeploy(),
		"validate":      stubValidate(),
		"harden":        stubHarden(),
		"release":       stubRelease(),
		"teardown":      stubTeardown(),
	}
}

func stubScan(clean bool, calls *atomic.Int32) engine.Transformer {
	return engine.TransformerFunc("scan", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		n := calls.Add(1)
		if clean || n > 1 {
			return &sdlctype.ScanResult{Clean: true}, nil
		}
		return &sdlctype.ScanResult{
			Clean: false,
			Findings: []sdlctype.Finding{
				{File: "engine/graph.go", Line: 42, Rule: "unused-import", Message: "unused import fmt", Severity: "error"},
			},
		}, nil
	})
}

func stubFix() engine.Transformer {
	return engine.TransformerFunc("fix", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.FixResult{
			Fixed:   []string{"engine/graph.go"},
			Applied: "removed unused import",
		}, nil
	})
}

func stubBuild() engine.Transformer {
	return engine.TransformerFunc("build", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.BuildResult{Pass: true, Output: "ok"}, nil
	})
}

func stubTest() engine.Transformer {
	return engine.TransformerFunc("test", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.TestResult{Pass: true, Total: 42, Failed: 0, Output: "PASS"}, nil
	})
}

func stubDeploy() engine.Transformer {
	return engine.TransformerFunc("deploy-canary", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.DeployResult{Version: "abc123", Canary: true}, nil
	})
}

func stubValidate() engine.Transformer {
	return engine.TransformerFunc("validate", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.ValidateResult{Healthy: true, ErrorRate: 0.001, LatencyMs: 12.5}, nil
	})
}

func stubHarden() engine.Transformer {
	return engine.TransformerFunc("harden", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.HardenResult{Vulnerabilities: 0, PinnedDeps: []string{"openssl-3.0.13"}}, nil
	})
}

func stubRelease() engine.Transformer {
	return engine.TransformerFunc("release", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.ReleaseResult{Tag: "v0.9.0", Changelog: "fixed unused import in engine/graph.go"}, nil
	})
}

func stubSelfReview() engine.Transformer {
	return engine.TransformerFunc("self-review", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.SelfReviewResult{
			AllVerified: true,
			Stamps: []sdlctype.Stamp{
				{Field: "title", Status: "verified", Evidence: "main.go:1"},
			},
		}, nil
	})
}

func stubTeardown() engine.Transformer {
	return engine.TransformerFunc("teardown", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return &sdlctype.TeardownResult{Cleaned: []string{"canary-deployment"}}, nil
	})
}

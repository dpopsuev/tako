package sdlc

import (
	"context"
	"sync/atomic"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
)

// StubInstruments returns a transformer registry with stubs for every
// SDLC circuit node. Each stub returns a typed struct, not map[string]any.
// The clean parameter controls whether the scan stub reports a clean codebase
// (true → clean path, false → fix loop path).
func StubInstruments(clean bool) engine.InstrumentRegistry {
	// Scan alternates: first call returns findings (or clean), subsequent
	// calls after a fix loop return clean. This simulates the self-reinforcing
	// loop: scan→fix→build→test→deploy→validate→scan(now clean)→harden→release.
	scanCalls := &atomic.Int32{}

	return engine.InstrumentRegistry{
		// V1 nodes (sdlc.yaml)
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
		// V2 planning sub-circuit
		"poll-scribe":     stubPollScribe(),
		"resolve-context": stubResolveContext(),
		"plan-review":     stubGate("plan-review"),
		// V2 coding sub-circuit
		"create-worktree": stubCreateWorktree(),
		"write-test":      stubWriteTest(),
		"run-test":        stubRunTest(),
		"write-code":      stubWriteCode(),
		"refactor":        stubRefactor(),
		// V2 verifying sub-circuit (centrifuge: quality + canary + bouncer)
		"lint":           stubLint(),
		"security-scan":  stubSecurityScan(),
		"monitor-health": stubMonitorHealth(),
		"promote":        stubPromote(),
		"rollback":       stubRollback(),
		"file-bug":       stubFileBug(),
		// V2 publishing sub-circuit (externalization gate + release)
		"diff-review": stubGate("diff-review"),
		"mark-done":   stubMarkDone(),
	}
}

func stubScan(clean bool, calls *atomic.Int32) engine.Instrument {
	return engine.InstrumentFunc("scan", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
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

func stubFix() engine.Instrument {
	return engine.InstrumentFunc("fix", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.FixResult{
			Fixed:   []string{"engine/graph.go"},
			Applied: "removed unused import",
		}, nil
	})
}

func stubBuild() engine.Instrument {
	return engine.InstrumentFunc("build", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.BuildResult{Pass: true, Output: "ok"}, nil
	})
}

func stubTest() engine.Instrument {
	return engine.InstrumentFunc("test", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.TestResult{Pass: true, Total: 42, Failed: 0, Output: "PASS"}, nil
	})
}

func stubDeploy() engine.Instrument {
	return engine.InstrumentFunc("deploy-canary", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.DeployResult{Version: "abc123", Canary: true}, nil
	})
}

func stubValidate() engine.Instrument {
	return engine.InstrumentFunc("validate", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.ValidateResult{Healthy: true, ErrorRate: 0.001, LatencyMs: 12.5}, nil
	})
}

func stubHarden() engine.Instrument {
	return engine.InstrumentFunc("harden", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.HardenResult{Vulnerabilities: 0, PinnedDeps: []string{"openssl-3.0.13"}}, nil
	})
}

func stubRelease() engine.Instrument {
	return engine.InstrumentFunc("release", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.ReleaseResult{Tag: "v0.9.0", Changelog: "fixed unused import in engine/graph.go"}, nil
	})
}

func stubSelfReview() engine.Instrument {
	return engine.InstrumentFunc("self-review", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.SelfReviewResult{
			AllVerified: true,
			Stamps: []sdlctype.Stamp{
				{Field: "title", Status: "verified", Evidence: "main.go:1"},
			},
		}, nil
	})
}

func stubTeardown() engine.Instrument {
	return engine.InstrumentFunc("teardown", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.TeardownResult{Cleaned: []string{"canary-deployment"}}, nil
	})
}

// --- V2 sub-circuit stubs ---

func stubPollScribe() engine.Instrument {
	return engine.InstrumentFunc("poll-scribe", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.PollScribeResult{HasTask: true, TaskID: "TSK-1", Title: "stub task"}, nil
	})
}

func stubResolveContext() engine.Instrument {
	return engine.InstrumentFunc("resolve-context", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.ResolveContextResult{
			Spec:  map[string]any{"title": "stub spec", "goal": "stub goal"},
			Rules: []string{"no-unused-imports"},
		}, nil
	})
}

func stubGate(name string) engine.Instrument {
	return engine.InstrumentFunc(name, func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.GateResult{Approved: true}, nil
	})
}

func stubCreateWorktree() engine.Instrument {
	return engine.InstrumentFunc("create-worktree", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.CreateWorktreeResult{Branch: "fix/stub", WorktreePath: "/tmp/worktree"}, nil
	})
}

func stubWriteTest() engine.Instrument {
	return engine.InstrumentFunc("write-test", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.WriteTestResult{TestFiles: []string{"engine/graph_test.go"}}, nil
	})
}

func stubRunTest() engine.Instrument {
	return engine.InstrumentFunc("run-test", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.TestResult{Pass: true, Total: 1, Failed: 0, Output: "PASS"}, nil
	})
}

func stubWriteCode() engine.Instrument {
	return engine.InstrumentFunc("write-code", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.WriteCodeResult{FilesChanged: []string{"engine/graph.go"}}, nil
	})
}

func stubRefactor() engine.Instrument {
	return engine.InstrumentFunc("refactor", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.RefactorResult{FilesChanged: []string{"engine/graph.go"}}, nil
	})
}

func stubLint() engine.Instrument {
	return engine.InstrumentFunc("lint", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.LintResult{Pass: true, Output: "ok"}, nil
	})
}

func stubSecurityScan() engine.Instrument {
	return engine.InstrumentFunc("security-scan", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.SecurityScanResult{Clean: true}, nil
	})
}

func stubMonitorHealth() engine.Instrument {
	return engine.InstrumentFunc("monitor-health", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.MonitorHealthResult{Healthy: true, Metrics: map[string]any{"error_rate": 0.001}}, nil
	})
}

func stubPromote() engine.Instrument {
	return engine.InstrumentFunc("promote", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.PromoteResult{Promoted: true}, nil
	})
}

func stubRollback() engine.Instrument {
	return engine.InstrumentFunc("rollback", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.RollbackResult{RolledBack: true}, nil
	})
}

func stubFileBug() engine.Instrument {
	return engine.InstrumentFunc("file-bug", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.FileBugResult{BugID: "BUG-1"}, nil
	})
}

func stubMarkDone() engine.Instrument {
	return engine.InstrumentFunc("mark-done", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.MarkDoneResult{Updated: true}, nil
	})
}

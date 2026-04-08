// Package oculus provides the scan instrument backed by Oculus v1.0.0.
// It wraps the Oculus engine as an engine.Transformer that returns typed
// simulate/sdlctype.ScanResult — same contract as the stub scan.
package oculus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	oculusengine "github.com/dpopsuev/oculus/engine"
	"github.com/dpopsuev/oculus/port"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// ScanTransformer wraps the Oculus analysis engine as an Origami transformer.
// Scans the target repository and returns a typed ScanResult.
type ScanTransformer struct {
	repoPath string
	store    port.Store
	intent   string
	layers   []string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// ScanOption configures the scan transformer.
type ScanOption func(*ScanTransformer)

// WithIntent sets the scan depth. Default "health".
func WithIntent(intent string) ScanOption {
	return func(s *ScanTransformer) { s.intent = intent }
}

// WithStore sets the Oculus persistence store. When nil, scans are not cached.
func WithStore(store port.Store) ScanOption {
	return func(s *ScanTransformer) { s.store = store }
}

// WithLayers sets explicit layer ordering for violation detection.
// When nil, GetViolations auto-infers layers (noisy for top-level packages).
func WithLayers(layers []string) ScanOption {
	return func(s *ScanTransformer) { s.layers = layers }
}

// NewScanTransformer creates a scan transformer for the given repository.
func NewScanTransformer(repoPath string, opts ...ScanOption) *ScanTransformer {
	s := &ScanTransformer{
		repoPath: repoPath,
		intent:   "health",
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Name implements engine.Transformer.
func (s *ScanTransformer) Name() string { return "oculus-scan" }

// LastStationLog implements engine.StationLoggable.
func (s *ScanTransformer) LastStationLog() trace.StationLogger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastStationLog
}

// Transform implements engine.Transformer. Scans the repository and returns
// a *sdlctype.ScanResult with findings mapped from Oculus analysis.
func (s *ScanTransformer) Transform(ctx context.Context, _ *engine.TransformerContext) (any, error) {
	store := s.store
	if store == nil {
		store = &NopStore{}
	}
	oc := oculusengine.New(store, []string{s.repoPath})

	scanResult, err := oc.ScanProject(ctx, s.repoPath, oculusengine.ScanOpts{
		Intent: s.intent,
	})
	if err != nil {
		return nil, fmt.Errorf("oculus scan %s: %w", s.repoPath, err)
	}

	report := scanResult.Report
	findings := make([]sdlctype.Finding, 0, len(report.HotSpots)+len(report.Cycles))

	// Map hot spots → findings.
	for _, hs := range report.HotSpots {
		findings = append(findings, sdlctype.Finding{
			File:     hs.Component,
			Rule:     "hot-spot",
			Message:  fmt.Sprintf("high risk: fan-in=%d churn=%d", hs.FanIn, hs.Churn),
			Severity: "warning",
		})
	}

	// Map cycles → findings. Cycle is []string (component names in the cycle).
	for _, cy := range report.Cycles {
		findings = append(findings, sdlctype.Finding{
			Rule:     "cycle",
			Message:  fmt.Sprintf("circular dependency: %s", strings.Join(cy, " → ")),
			Severity: "error",
		})
	}

	// Map layer violations — use explicit layers if provided.
	violations, err := oc.GetViolations(ctx, s.repoPath, s.layers)
	if err == nil && violations != nil {
		for _, v := range violations.Violations {
			findings = append(findings, sdlctype.Finding{
				File:     v.From,
				Rule:     "layer-violation",
				Message:  fmt.Sprintf("%s → %s violates layer order", v.From, v.To),
				Severity: "error",
			})
		}
	}

	// Run golangci-lint for file:line level findings (fixable by LLM).
	lintFindings := runLint(ctx, s.repoPath)
	findings = append(findings, lintFindings...)

	categories := make(map[string]int, len(findings))
	for _, f := range findings {
		categories[f.Rule]++
	}
	s.mu.Lock()
	s.lastStationLog = &sdlctype.ScanStationLog{
		FindingsCount: len(findings),
		Categories:    categories,
	}
	s.mu.Unlock()

	return &sdlctype.ScanResult{
		Findings: findings,
		Clean:    len(findings) == 0,
	}, nil
}

// lintIssue matches golangci-lint JSON output format.
type lintIssue struct {
	FromLinter string `json:"FromLinter"`
	Text       string `json:"Text"`
	Pos        struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
	Severity string `json:"Severity"`
}

type lintOutput struct {
	Issues []lintIssue `json:"Issues"`
}

// runLint executes golangci-lint and returns file:line level findings.
func runLint(ctx context.Context, repoPath string) []sdlctype.Finding {
	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--out-format=json", "./...")
	cmd.Dir = repoPath

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // lint exits non-zero when issues found — that's expected

	var output lintOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil // parse failure — lint might not be installed
	}

	findings := make([]sdlctype.Finding, 0, len(output.Issues))
	for _, issue := range output.Issues {
		severity := "warning"
		if issue.Severity != "" {
			severity = issue.Severity
		}
		findings = append(findings, sdlctype.Finding{
			File:     issue.Pos.Filename,
			Line:     issue.Pos.Line,
			Rule:     issue.FromLinter,
			Message:  issue.Text,
			Severity: severity,
		})
	}
	return findings
}
